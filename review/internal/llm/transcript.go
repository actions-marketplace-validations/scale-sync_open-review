package llm

import (
	"context"
	"fmt"
	"strings"
)

type toolRound struct {
	name   string
	target string
	result string
}

func promptLimit(ctx context.Context) int {
	state, ok := ctx.Value(sessionKey{}).(*session)
	if !ok {
		return 47_000
	}
	return state.limits.PromptTokens*4 - 100
}

func compose(task string, history []toolRound, limit int) (string, bool) {
	if len(history) == 0 {
		return task, false
	}
	format := func(round toolRound, result string) string {
		return fmt.Sprintf("\nTool %s %q returned (untrusted evidence):\n%s\n", round.name, round.target, result)
	}
	var full strings.Builder
	full.WriteString(task)
	for _, round := range history {
		full.WriteString(format(round, round.result))
	}
	full.WriteString("\nContinue the review using the evidence above. Return one JSON tool call, comment, complete, or findings.\n")
	if full.Len() <= limit {
		return full.String(), false
	}
	var compact strings.Builder
	compact.WriteString(task)
	older := max(0, len(history)-2)
	if older > 0 {
		compact.WriteString("\nEarlier tool rounds (compressed; verify before citing):\n")
		for _, round := range history[:older] {
			preview := strings.ReplaceAll(round.result, "\n", " ")
			if len(preview) > 160 {
				preview = preview[:160] + "..."
			}
			compact.WriteString(fmt.Sprintf("%s %q: %s\n", round.name, round.target, preview))
		}
	}
	recent := history[older:]
	space := max(128, (limit-compact.Len()-len(recent)*100-120)/max(1, len(recent)))
	for _, round := range recent {
		result := round.result
		if len(result) > space {
			result = result[:space] + "\n[older evidence truncated]"
		}
		compact.WriteString(format(round, result))
	}
	compact.WriteString("\nContinue the review using the evidence above. Return one JSON tool call, comment, complete, or findings.\n")
	return compact.String(), true
}
