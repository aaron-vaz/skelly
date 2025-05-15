package main

import (
	"os"

	"github.com/aaron-vaz/proj/internal/cli"
	"github.com/aaron-vaz/proj/internal/commands"
	"github.com/aaron-vaz/proj/internal/download"
	"github.com/aaron-vaz/proj/internal/templates"
	"github.com/aaron-vaz/proj/internal/view"
)

type Proj struct {
	downloader download.Downloader
	renderer   *templates.RendererService
	invoker    commands.Invoker
}

func (p *Proj) Run() error {
	return p.invoker.Execute(os.Args)
}

func main() {
	downloader := download.NewGoGetterDownloader()
	renderer := templates.NewRendererService()
	ui := view.NewStdUI(os.Stdin, os.Stdout, os.Stderr)
	invoker := cli.NewFlagCommandInvoker(downloader, renderer, ui)
	proj := &Proj{
		downloader: downloader,
		renderer:   renderer,
		invoker:    invoker,
	}

	if err := proj.Run(); err != nil {
		ui.RenderError(err.Error() + "\n")
		os.Exit(1)
	}
}
