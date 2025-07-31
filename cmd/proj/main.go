package main

import (
	"fmt"
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
