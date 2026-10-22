// Package openreview exposes the Go review engine as an importable API.
package openreview

import (
	"context"

	"github.com/scale-sync/open-review/review"
)

const ContractVersion = 2

type Options = review.Options
type Request = review.Request
type Result = review.Result
type Finding = review.Finding
type CoverageEntry = review.CoverageEntry
type ModelIdentity = review.ModelIdentity
type Limits = review.Limits
type ProviderConfig = review.ProviderConfig

// ConfigureProvider replaces the saved BYOK provider and key.
func ConfigureProvider(value ProviderConfig) error { return review.SaveProviderConfig(value) }

// Prepare selects reviewable files and resolves repository guidance.
func Prepare(ctx context.Context, options Options) (Request, error) {
	return review.Prepare(ctx, options)
}

// Run executes a prepared request with the same engine used by the CLI.
func Run(ctx context.Context, request Request) (Result, error) {
	return review.Run(ctx, request)
}

// Review prepares and executes a review. Use options.Mode "workspace", "commit",
// "branch", or "scan" and set the corresponding ref or path fields.
func Review(ctx context.Context, options Options, model ModelIdentity, limits Limits) (Result, error) {
	request, err := Prepare(ctx, options)
	if err != nil {
		return Result{}, err
	}
	request.Model = model
	request.Limits = limits
	return Run(ctx, request)
}

// Scan reviews a file or directory using the same review engine.
func Scan(ctx context.Context, repo, path string, model ModelIdentity, limits Limits) (Result, error) {
	return Review(ctx, Options{Repo: repo, Mode: "scan", ScanPath: path}, model, limits)
}
