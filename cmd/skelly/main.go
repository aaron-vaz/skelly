package main

import (
	"fmt"
	"os"

	"github.com/aaron-vaz/skelly/internal/cli"
	"github.com/aaron-vaz/skelly/internal/commands"
	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
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
	processor := templates.NewTemplateProcessor(renderer)
	ui := view.NewStdUI(os.Stdin, os.Stdout, os.Stderr)
	invoker := cli.NewFlagCommandInvoker(downloader, processor, ui)
	proj := &Proj{
		downloader: downloader,
		renderer:   renderer,
		invoker:    invoker,
	}

	if err := proj.Run(); err != nil {
		_ = ui.RenderError(fmt.Sprintf("Issue running proj: %s", err.Error()))
		os.Exit(1)
	}
}
