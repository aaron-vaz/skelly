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
	view := view.NewStdInputView()
	invoker := cli.NewFlagCommandInvoker(downloader, renderer, view)
	proj := &Proj{
		downloader: downloader,
		renderer:   renderer,
		invoker:    invoker,
	}

	if err := proj.Run(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
