package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aaron-vaz/skelly/internal/cli"
	"github.com/aaron-vaz/skelly/internal/config"
	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	store := config.NewFileStore(configPath)

	downloader := download.NewSmartDownloader(
		download.NewLocalDownloader(),
		download.NewGoGetterDownloader(),
	)
	renderer := templates.NewRendererService()
	processor := templates.NewTemplateProcessor(renderer)
	ui := view.NewStdUI(os.Stdin, os.Stdout, os.Stderr)
	invoker := cli.NewFlagCommandInvoker(downloader, processor, store, ui)

	// Trapping the first Ctrl+C lets init stop and remove its working directory;
	// once it fires, the default handler is restored so a second Ctrl+C still
	// kills the process outright.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)

	return invoker.Execute(ctx, os.Args[1:])
}
