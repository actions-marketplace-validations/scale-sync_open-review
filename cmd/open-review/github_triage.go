package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/scale-sync/open-review/review"
)

type triageFinding struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Title     string `json:"title"`
}

func sessionFinding(record review.SessionRecord, id string) (triageFinding, error) {
	var result struct {
		Findings []triageFinding `json:"findings"`
	}
	if err := json.Unmarshal(record.Result, &result); err != nil {
		return triageFinding{}, err
	}
	for _, finding := range result.Findings {
		if finding.ID == id {
			return finding, nil
		}
	}
	return triageFinding{}, fmt.Errorf("finding %q is not in this session", id)
}

var pullURL = regexp.MustCompile(`^/[^/]+/[^/]+/pull/[1-9][0-9]*$`)

func syncGitHubThread(client *http.Client, endpoint, token, prURL, threadID string, finding triageFinding, state string) error {
	if state != "open" && state != "fixed" && state != "ignored" && state != "false_positive" {
		return errors.New("invalid triage state")
	}
	if token == "" || threadID == "" || finding.ID == "" || finding.File == "" || finding.Title == "" {
		return errors.New("GitHub sync needs a token, thread, and complete finding")
	}
	parsed, err := url.Parse(prURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" || !pullURL.MatchString(parsed.Path) || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid GitHub pull request URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var current struct {
		Node *struct {
			ID                 string `json:"id"`
			Path               string `json:"path"`
			Line               *int   `json:"line"`
			IsResolved         bool   `json:"isResolved"`
			ViewerCanResolve   bool   `json:"viewerCanResolve"`
			ViewerCanUnresolve bool   `json:"viewerCanUnresolve"`
			PullRequest        struct {
				URL string `json:"url"`
			} `json:"pullRequest"`
			Comments struct {
				Nodes []struct {
					Body string `json:"body"`
				} `json:"nodes"`
			} `json:"comments"`
		} `json:"node"`
	}
	query := `query($id:ID!){node(id:$id){... on PullRequestReviewThread{id path line isResolved viewerCanResolve viewerCanUnresolve pullRequest{url} comments(first:1){nodes{body}}}}}`
	if err := githubGraphQL(ctx, client, endpoint, token, query, map[string]string{"id": threadID}, &current); err != nil {
		return err
	}
	thread := current.Node
	line := finding.EndLine
	if line == 0 {
		line = finding.StartLine
	}
	if thread == nil || thread.ID != threadID || thread.PullRequest.URL != strings.TrimSuffix(prURL, "/") || thread.Path != finding.File || thread.Line == nil || *thread.Line != line || len(thread.Comments.Nodes) == 0 || !strings.Contains(thread.Comments.Nodes[0].Body, finding.Title) {
		return errors.New("GitHub thread does not match the selected finding and pull request")
	}
	resolved := state != "open"
	if thread.IsResolved == resolved {
		return nil
	}
	if resolved && !thread.ViewerCanResolve || !resolved && !thread.ViewerCanUnresolve {
		return errors.New("token cannot change this GitHub thread")
	}
	mutation := `mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{id isResolved}}}`
	if !resolved {
		mutation = `mutation($id:ID!){unresolveReviewThread(input:{threadId:$id}){thread{id isResolved}}}`
	}
	var changed struct {
		Resolve struct {
			Thread struct {
				ID         string `json:"id"`
				IsResolved bool   `json:"isResolved"`
			} `json:"thread"`
		} `json:"resolveReviewThread"`
		Unresolve struct {
			Thread struct {
				ID         string `json:"id"`
				IsResolved bool   `json:"isResolved"`
			} `json:"thread"`
		} `json:"unresolveReviewThread"`
	}
	if err := githubGraphQL(ctx, client, endpoint, token, mutation, map[string]string{"id": threadID}, &changed); err != nil {
		return err
	}
	updated := changed.Resolve.Thread
	if !resolved {
		updated = changed.Unresolve.Thread
	}
	if updated.ID != threadID || updated.IsResolved != resolved {
		return errors.New("GitHub thread did not reach the requested state")
	}
	return nil
}

func githubGraphQL(ctx context.Context, client *http.Client, endpoint, token, query string, variables map[string]string, target any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "open-review")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub GraphQL returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("GitHub GraphQL: %s", envelope.Errors[0].Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("GitHub GraphQL returned no data")
	}
	return json.Unmarshal(envelope.Data, target)
}
