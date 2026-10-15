package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/scale-sync/open-review/review"
)

func runSessions(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: open-review sessions list|show|inspect|compare|export|delete|triage|serve")
	}
	command := args[0]
	flags := flag.NewFlagSet("sessions "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("dir", "", "session directory")
	repo := flags.String("repo", ".", "repository path")
	file := flags.String("output", "", "export path")
	addr := flags.String("addr", "127.0.0.1:0", "loopback viewer address")
	prURL := flags.String("pr-url", "", "GitHub pull request URL for explicit thread sync")
	threadID := flags.String("thread-id", "", "GitHub review thread node ID")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	if *dir == "" {
		*dir, err = review.SessionDir()
		if err != nil {
			return err
		}
	}
	positions := flags.Args()
	encode := func(value any) error {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		return writeOutput(output, *file, append(data, '\n'))
	}
	switch command {
	case "serve":
		if len(positions) != 0 || *file != "" {
			return errors.New("usage: sessions serve [--dir PATH] [--repo PATH] [--addr 127.0.0.1:PORT]")
		}
		host, _, err := net.SplitHostPort(*addr)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return errors.New("session viewer must bind to a numeric loopback address")
		}
		listener, err := net.Listen("tcp", *addr)
		if err != nil {
			return err
		}
		defer listener.Close()
		if _, err := fmt.Fprintf(output, "Session viewer: http://%s/\n", listener.Addr()); err != nil {
			return err
		}
		return http.Serve(listener, review.SessionViewer(*dir, root))
	case "list":
		if len(positions) != 0 || *file != "" {
			return errors.New("usage: sessions list [--dir PATH] [--repo PATH]")
		}
		items, err := review.ListSessions(*dir, root)
		if err != nil {
			return err
		}
		return encode(items)
	case "triage":
		if len(positions) != 3 || *file != "" {
			return errors.New("usage: sessions triage [flags] ID FINDING_ID open|fixed|ignored|false_positive")
		}
		if (*prURL == "") != (*threadID == "") {
			return errors.New("GitHub sync requires both --pr-url and --thread-id")
		}
		if *prURL != "" {
			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				token = os.Getenv("GH_TOKEN")
			}
			if token == "" {
				return errors.New("GitHub sync requires GITHUB_TOKEN or GH_TOKEN")
			}
			record, err := review.ReadSession(*dir, positions[0], root)
			if err != nil {
				return err
			}
			finding, err := sessionFinding(record, positions[1])
			if err != nil {
				return err
			}
			if err := syncGitHubThread(http.DefaultClient, "https://api.github.com/graphql", token, *prURL, *threadID, finding, positions[2]); err != nil {
				return err
			}
		}
		states, err := review.SetFindingTriage(*dir, positions[0], root, positions[1], positions[2])
		if err != nil {
			return err
		}
		return encode(states)
	case "show", "inspect", "export", "delete":
		if len(positions) != 1 {
			return fmt.Errorf("usage: sessions %s [flags] ID", command)
		}
		if command == "delete" {
			if err := review.DeleteSession(*dir, positions[0], root); err != nil {
				return err
			}
			return encode(map[string]string{"deleted": positions[0]})
		}
		record, err := review.ReadSession(*dir, positions[0], root)
		if err != nil {
			return err
		}
		if command == "inspect" {
			return encode(record.Events)
		}
		if command == "export" && *file == "" {
			return errors.New("sessions export requires --output")
		}
		return encode(record)
	case "compare":
		if len(positions) != 2 || *file != "" {
			return errors.New("usage: sessions compare [flags] ID ID")
		}
		left, err := review.ReadSession(*dir, positions[0], root)
		if err != nil {
			return err
		}
		right, err := review.ReadSession(*dir, positions[1], root)
		if err != nil {
			return err
		}
		comparison, err := compareSessions(left, right)
		if err != nil {
			return err
		}
		return encode(comparison)
	default:
		return errors.New("usage: open-review sessions list|show|inspect|compare|export|delete|triage|serve")
	}
}

func compareSessions(left, right review.SessionRecord) (map[string]any, error) {
	type outcome struct {
		Status   string `json:"status"`
		Findings []struct {
			ID string `json:"id"`
		} `json:"findings"`
	}
	var before, after outcome
	if len(left.Result) == 0 || len(right.Result) == 0 {
		return nil, errors.New("both sessions need completed result files")
	}
	if err := json.Unmarshal(left.Result, &before); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(right.Result, &after); err != nil {
		return nil, err
	}
	old := map[string]bool{}
	newer := map[string]bool{}
	for _, finding := range before.Findings {
		old[finding.ID] = true
	}
	for _, finding := range after.Findings {
		newer[finding.ID] = true
	}
	added, removed := []string{}, []string{}
	for id := range newer {
		if !old[id] {
			added = append(added, id)
		}
	}
	for id := range old {
		if !newer[id] {
			removed = append(removed, id)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	return map[string]any{"before": left.Summary.ID, "after": right.Summary.ID, "beforeStatus": before.Status, "afterStatus": after.Status, "addedFindings": added, "removedFindings": removed}, nil
}
