package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yunzhu457/CCode/internal/chat"
	"github.com/yunzhu457/CCode/internal/config"
	"github.com/yunzhu457/CCode/internal/llm"
	"github.com/yunzhu457/CCode/internal/tui"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ccode: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("ccode", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "configs/config.local.yaml", "path to YAML config")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	client, err := llm.NewFromConfig(cfg)
	if err != nil {
		return err
	}

	app := tui.New(stdin, stdout, chat.NewSession(), client)
	return app.Run(context.Background())
}
