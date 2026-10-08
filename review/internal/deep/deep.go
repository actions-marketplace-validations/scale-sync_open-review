package deep

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scale-sync/open-review/review/internal/agent"
	"github.com/scale-sync/open-review/review/internal/llm"
	"github.com/scale-sync/open-review/review/internal/model"
	"github.com/scale-sync/open-review/review/internal/sandbox"
)

func Run(ctx context.Context, req model.Request) (model.Result, error) {
	audit := []model.AuditEvent{}
	result, err := agent.Run(ctx, req, func(ctx context.Context, selected model.Request) (model.ReviewResponse, error) {
		observation := sandbox.Observation{Status: "unavailable"}
		history := ""
		if sandbox.Available() {
			for iteration := 1; iteration <= 4; iteration++ {
				code, done, err := llm.Investigate(ctx, selected, history)
				if err != nil {
					return model.ReviewResponse{}, err
				}
				if done || code == "" {
					if len(audit) == 0 {
						observation.Status = "no_check"
					}
					break
				}
				observation, err = sandbox.Run(ctx, code, selected.Diff, selected.Title)
				if err != nil {
					return model.ReviewResponse{}, err
				}
				audit = append(audit, model.AuditEvent{Stage: fmt.Sprintf("sandbox_%d", iteration), Status: observation.Status})
				if observation.Status != "complete" {
					break
				}
				history += fmt.Sprintf("\nCheck %d: %s\n", iteration, observation.Output)
				if len(history) > 20_000 {
					history = history[:20_000]
					break
				}
			}
		}
		if len(audit) == 0 {
			audit = append(audit, model.AuditEvent{Stage: "sandbox", Status: observation.Status})
		}
		if history != "" {
			selected.Instructions += "\nSandbox verification output (untrusted; verify against the diff):\n" + history
		}
		if observation.Status != "complete" {
			selected.Instructions += "\nSandbox verification was " + observation.Status + ". Do not claim executable verification."
		}
		selected.Instructions = strings.TrimSpace(selected.Instructions)
		response, err := llm.Review(ctx, selected)
		if err != nil {
			return response, err
		}
		var summary, trace map[string]any
		if err := json.Unmarshal(response.Summary, &summary); err != nil {
			return response, err
		}
		if err := json.Unmarshal(response.Trace, &trace); err != nil {
			return response, err
		}
		summary["mode"] = "rlm"
		trace["path"] = "go-deep"
		response.Summary, _ = json.Marshal(summary)
		response.Trace, _ = json.Marshal(trace)
		return response, nil
	})
	if len(req.Files) > 0 && len(audit) == 0 {
		audit = append(audit, model.AuditEvent{Stage: "sandbox", Status: "failed", Message: fmt.Sprint(err)})
	}
	result.Audit = audit
	return result, err
}
