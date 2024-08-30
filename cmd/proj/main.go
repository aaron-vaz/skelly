package main

import (
	"flag"
	"os"

	"github.com/aaron-vaz/proj/internal/commands"
	"github.com/aaron-vaz/proj/internal/templates"
)

type Proj struct {
	renderer *templates.RendererService
}

func (p *Proj) Run() error {
	flag.Parse()

	switch flag.Arg(0) {
	case "init":
		init := flag.NewFlagSet("init", flag.ExitOnError)
		src := init.String("src", "", "URL to template that will be used to init the new project")
		dst := init.String("dst", ".", "Destination dir where the project will be initialised to (Defaults to current directory)")

		err := init.Parse(flag.Args()[1:])
		if err != nil {
			return err
		}

		return commands.NewInitCommand(*src, *dst, p.renderer).Execute()
	default:
		flag.Usage()
	}

	return nil
}

func main() {
	renderer := templates.NewRendererService()
	proj := &Proj{
		renderer: renderer,
	}

	if err := proj.Run(); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
