package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/scale-sync/open-review/review"
)

func runLearnings(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: open-review learnings list|add|delete|export|import")
	}
	command := args[0]
	flags := flag.NewFlagSet("learnings "+command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("dir", "", "learnings directory")
	repo := flags.String("repo", ".", "repository path or slug")
	asJSON := flags.Bool("json", false, "format output as JSON")
	trigger := flags.String("trigger", "", "trigger phrase or pattern")
	finding := flags.String("finding", "", "finding or guidance text")
	source := flags.String("source", "manual", "source of learning: manual, feedback, triage")
	days := flags.Int("days", 90, "days before expiration")
	outFile := flags.String("output", "", "output file path for export")

	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		root = *repo
	}

	positions := flags.Args()
	switch command {
	case "list":
		items, err := review.ListLearnings(*dir, root)
		if err != nil {
			return err
		}
		if *asJSON {
			enc := json.NewEncoder(output)
			enc.SetIndent("", "  ")
			return enc.Encode(items)
		}
		if len(items) == 0 {
			fmt.Fprintln(output, review.T("learnings.empty"))
			return nil
		}
		for _, item := range items {
			fmt.Fprintf(output, "[%s] (%s) %s -> %s (used %d times)\n", item.ID, item.Source, item.Trigger, item.Finding, item.UsedCount)
		}
		return nil

	case "add":
		if *trigger == "" || *finding == "" {
			return errors.New("both --trigger and --finding are required")
		}
		item, err := review.AddLearning(*dir, root, *trigger, *finding, *source, *days)
		if err != nil {
			return err
		}
		if *asJSON {
			enc := json.NewEncoder(output)
			enc.SetIndent("", "  ")
			return enc.Encode(item)
		}
		fmt.Fprintln(output, review.T("learnings.added", item.ID, root))
		return nil

	case "delete":
		if len(positions) == 0 {
			return errors.New("usage: open-review learnings delete <id>")
		}
		id := positions[0]
		if err := review.DeleteLearning(*dir, root, id); err != nil {
			return err
		}
		fmt.Fprintln(output, review.T("learnings.deleted", id))
		return nil

	case "export":
		data, err := review.ExportLearnings(*dir, root)
		if err != nil {
			return err
		}
		return writeOutput(output, *outFile, append(data, '\n'))

	case "import":
		if len(positions) == 0 {
			return errors.New("usage: open-review learnings import <file>")
		}
		content, err := os.ReadFile(positions[0])
		if err != nil {
			return err
		}
		count, err := review.ImportLearnings(*dir, root, content)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "Imported %d learnings into %s\n", count, root)
		return nil

	default:
		return fmt.Errorf("unknown learnings command %q", command)
	}
}
