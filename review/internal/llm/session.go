package llm

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/scale-sync/open-review/review/internal/model"
)

var ErrBudget = errors.New("model token budget exhausted")

type sessionKey struct{}

type session struct {
	mu     sync.Mutex
	limits model.Limits
	usage  model.Usage
}

func WithSession(parent context.Context, limits model.Limits) (context.Context, context.CancelFunc, func() model.Usage, error) {
	if limits.PromptTokens < 0 || limits.OutputTokens < 0 || limits.TotalTokens < 0 || limits.RequestSeconds < 0 || limits.GroupSeconds < 0 {
		return parent, nil, nil, errors.New("model limits cannot be negative")
	}
	if limits.PromptTokens > 1_000_000 || limits.OutputTokens > 100_000 || limits.TotalTokens > 10_000_000 || limits.RequestSeconds > 3_600 || limits.GroupSeconds > 86_400 {
		return parent, nil, nil, errors.New("model limits exceed supported maximums")
	}
	if limits.PromptTokens == 0 {
		limits.PromptTokens = 12_000
	}
	if limits.OutputTokens == 0 {
		limits.OutputTokens = 2_048
	}
	if limits.TotalTokens == 0 {
		limits.TotalTokens = 30_000
	}
	if limits.RequestSeconds == 0 {
		limits.RequestSeconds = 90
	}
	if limits.GroupSeconds == 0 {
		limits.GroupSeconds = 300
	}
	state := &session{limits: limits}
	ctx, cancel := context.WithTimeout(context.WithValue(parent, sessionKey{}, state), time.Duration(limits.GroupSeconds)*time.Second)
	return ctx, cancel, func() model.Usage { state.mu.Lock(); defer state.mu.Unlock(); return state.usage }, nil
}

func budget(ctx context.Context, promptBytes int) (int, time.Duration, error) {
	state, ok := ctx.Value(sessionKey{}).(*session)
	if !ok {
		return 2_048, 90 * time.Second, nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	estimate := max(1, (promptBytes+3)/4)
	remaining := state.limits.TotalTokens - state.usage.TotalTokens - estimate
	if estimate > state.limits.PromptTokens || remaining < 1 {
		return 0, 0, ErrBudget
	}
	return min(state.limits.OutputTokens, remaining), time.Duration(state.limits.RequestSeconds) * time.Second, nil
}

func record(ctx context.Context, promptBytes, outputBytes, promptTokens, outputTokens, outputCap int) bool {
	state, ok := ctx.Value(sessionKey{}).(*session)
	if !ok {
		return false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if promptTokens < 1 {
		promptTokens = max(1, (promptBytes+3)/4)
		state.usage.Estimated = true
	}
	if outputTokens < 1 {
		outputTokens = max(1, (outputBytes+3)/4)
		state.usage.Estimated = true
	}
	state.usage.PromptTokens += promptTokens
	state.usage.OutputTokens += outputTokens
	state.usage.TotalTokens += promptTokens + outputTokens
	state.usage.ModelCalls++
	return promptTokens > state.limits.PromptTokens || outputTokens > outputCap || state.usage.TotalTokens > state.limits.TotalTokens
}
