package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/scale-sync/open-review/review"
)

func runRules(args []string, output io.Writer) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New("usage: open-review rules check --file PATH [--repo PATH] [--output text|json]")
	}
	flags := flag.NewFlagSet("rules check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repo := flags.String("repo", ".", "repository path")
	file := flags.String("file", "", "repository-relative file")
	format := flags.String("output", "text", "text or json")
	instruction := flags.String("instruction", "", "explicit review instruction")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *file == "" || *format != "text" && *format != "json" || len(*instruction) > 8_000 {
		return errors.New("rules check requires --file, text or json output, and an instruction under 8 KB")
	}
	rules, err := review.CheckRules(*repo, []string{*file}, *instruction)
	if err != nil {
		return err
	}
	if *format == "json" {
		return json.NewEncoder(output).Encode(rules[0])
	}
	_, err = fmt.Fprintf(output, "%s\nSources: %s\n\n%s\n", rules[0].Path, strings.Join(rules[0].Sources, " -> "), rules[0].Text)
	return err
}
