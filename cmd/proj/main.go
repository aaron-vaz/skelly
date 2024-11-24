package main

import (
	"os"

	"github.com/aaron-vaz/proj/internal/cli"
	"github.com/aaron-vaz/proj/internal/commands"
	"github.com/aaron-vaz/proj/internal/templates"
)

type Proj struct {
	renderer *templates.RendererService
	invoker  commands.Invoker
}

func (p *Proj) Run() error {
	return p.invoker.Execute(os.Args)
}

func main() {
	renderer := templates.NewRendererService()
	invoker := cli.NewFlagCommandInvoker(renderer)
	proj := &Proj{
		renderer: renderer,
		invoker:  invoker,
	}

	if err := proj.Run(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
