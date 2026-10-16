package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/scale-sync/open-review/review"
)

func runSARIF(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("sarif", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	input := flags.String("input", "", "review JSON input")
	file := flags.String("output", "", "SARIF output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("usage: open-review sarif --input review.json [--output report.sarif]")
	}
	reader, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, (20<<20)+1))
	if err != nil {
		return err
	}
	if len(raw) > 20<<20 {
		return errors.New("review JSON exceeds 20 MB")
	}
	var result review.Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	data, err := review.FormatSARIF(result)
	if err != nil {
		return err
	}
	return writeOutput(output, *file, append(data, '\n'))
}
