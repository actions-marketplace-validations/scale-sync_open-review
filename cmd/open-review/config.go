package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/scale-sync/open-review/review"
)

func runConfig(ctx context.Context, args []string, output io.Writer) error {
	return runConfigInput(ctx, args, os.Stdin, output)
}

func runConfigInput(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: open-review config setup|set|show|test")
	}
	switch args[0] {
	case "show":
		if len(args) != 1 {
			return errors.New("config show takes no arguments")
		}
		saved, err := review.LoadProviderConfig()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "Provider: %s\nModel: %s\nBase URL: %s\nSaved key: %t\n", saved.Provider, saved.Model, saved.BaseURL, saved.APIKey != "")
		return err
	case "test":
		flags := flag.NewFlagSet("config test", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		provider := flags.String("provider", "", "provider override")
		name := flags.String("model", "", "model override")
		baseURL := flags.String("base-url", "", "endpoint override")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected config test arguments")
		}
		identity := review.ModelIdentity{Provider: *provider, Name: *name, BaseURL: *baseURL}
		if err := review.TestProvider(ctx, identity); err != nil {
			return err
		}
		selected, err := review.ResolveProviderConfig(identity)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "Connected to %s (%s)\n", selected.Provider, selected.Model)
		return err
	case "setup", "set":
		flags := flag.NewFlagSet("config "+args[0], flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		provider := flags.String("provider", "", "openai, anthropic, gemini, openrouter, azure, or bedrock")
		name := flags.String("model", "", "model name")
		baseURL := flags.String("base-url", "", "custom HTTPS endpoint")
		keyStdin := flags.Bool("api-key-stdin", false, "read API key from standard input")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("unexpected config arguments")
		}
		if args[0] == "setup" && !*keyStdin {
			scanner := bufio.NewScanner(input)
			if *provider == "" {
				fmt.Fprint(output, "Provider (openai/anthropic/gemini/openrouter/azure/bedrock): ")
				if !scanner.Scan() {
					return errors.New("provider input required")
				}
				*provider = strings.TrimSpace(scanner.Text())
			}
			if *name == "" {
				fmt.Fprint(output, "Model: ")
				if !scanner.Scan() {
					return errors.New("model input required")
				}
				*name = strings.TrimSpace(scanner.Text())
			}
		}
		if *provider == "" || *name == "" {
			return errors.New("provider and model are required")
		}
		saved := review.ProviderConfig{Provider: *provider, Model: *name, BaseURL: *baseURL}
		if previous, err := review.LoadProviderConfig(); err != nil {
			return err
		} else if previous.Provider == saved.Provider {
			saved.APIKey = previous.APIKey
		}
		if *keyStdin {
			raw, err := io.ReadAll(io.LimitReader(input, 8_193))
			if err != nil {
				return err
			}
			if len(raw) > 8_192 {
				return errors.New("API key is too long")
			}
			saved.APIKey = strings.TrimSpace(string(raw))
			if saved.APIKey == "" {
				return errors.New("API key is empty")
			}
		}
		if err := review.SaveProviderConfig(saved); err != nil {
			return err
		}
		path, err := review.ProviderConfigPath()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "Saved %s (%s) to %s. Set the provider API key environment variable or use --api-key-stdin.\n", saved.Provider, saved.Model, path)
		return err
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}
