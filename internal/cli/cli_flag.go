package cli

import (
	"flag"

	"github.com/aaron-vaz/proj/internal/commands"
	"github.com/aaron-vaz/proj/internal/download"
	"github.com/aaron-vaz/proj/internal/templates"
)

type FlagCommandInvoker struct {
	downloader download.Downloader
	renderer   *templates.RendererService
}

func (i *FlagCommandInvoker) Execute(args []string) error {
	switch args[1] {
	case "init":
		init := flag.NewFlagSet("init", flag.ExitOnError)
		src := init.String("src", "", "URL to template that will be used to init the new project")
		dst := init.String("dst", ".", "Destination dir where the project will be initialised to (Defaults to current directory)")

		err := init.Parse(args[2:])
		if err != nil {
			return err
		}

		options := commands.InitOptions{
			Source:      *src,
			Destination: *dst,
		}

		return commands.NewInitCommand(i.downloader, i.renderer, options).Execute()
	case "help":
	default:
		flag.Usage()
	}

	return nil
}

func NewFlagCommandInvoker(downloader download.Downloader, renderer *templates.RendererService) commands.Invoker {
	return &FlagCommandInvoker{
		downloader: downloader,
		renderer:   renderer,
	}
}
