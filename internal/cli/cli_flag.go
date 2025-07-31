package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/aaron-vaz/skelly/internal/commands"
	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

const (
	cmdInit    = "init"
	cmdHelp    = "help"
	cmdVersion = "version"
)

// Version can be injected at build time using -ldflags "-X github.com/aaron-vaz/skelly/internal/cli.version=x.y.z"
var version = "dev"

type FlagCommandInvoker struct {
	downloader download.Downloader
	processor  *templates.Processor
	ui         view.UI
}

func (i *FlagCommandInvoker) Execute(args []string) error {
	if len(args) < 2 {
		i.showHelp("")
		return errors.New("no command provided")
	}

	switch args[1] {
	case cmdVersion:
		return i.ui.RenderInfo(fmt.Sprintf("skelly version %s", version))
	case cmdInit:
		init := flag.NewFlagSet(cmdInit, flag.ExitOnError)
		src := init.String("src", "", "URL to template that will be used to init the new project (required)")
		dst := init.String("dst", ".", "Destination dir where the project will be initialised to (Defaults to current directory)")

		if err := init.Parse(args[2:]); err != nil {
			init.Usage()
			return fmt.Errorf("failed to parse init flags: %w", err)
		}

		if *src == "" {
			init.Usage()
			return errors.New("required flag -src not provided")
		}

		options := commands.InitOptions{
			Source:      *src,
			Destination: *dst,
		}

		return commands.NewInitCommand(i.processor, i.downloader, options, i.ui).Execute()
	case cmdHelp:
		command := ""
		if len(args) > 2 {
			command = args[2]
		}
		i.showHelp(command)
		return nil
	default:
		i.showHelp("")
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func (i *FlagCommandInvoker) showHelp(command string) {
	switch command {
	case cmdInit:
		fmt.Println("USAGE:")
		fmt.Println("    skelly init --src SOURCE [--dst DEST]")
		fmt.Println("")
		fmt.Println("ARGS:")
		fmt.Println("    SOURCE    URL of the template repository")
		fmt.Println("    DEST      Destination directory (default: current directory)")
		fmt.Println("")
		fmt.Println("EXAMPLES:")
		fmt.Println("    $ skelly init --src https://github.com/user/template")
		fmt.Println("    $ skelly init --src https://github.com/user/template --dst ./my-project")
	default:
		fmt.Println("USAGE:")
		fmt.Println("    skelly COMMAND [OPTIONS]")
		fmt.Println("")
		fmt.Println("COMMANDS:")
		fmt.Println("    init        Initialize a new project from a template")
		fmt.Println("    help        Show command help")
		fmt.Println("    version     Show skelly version")
		fmt.Println("")
		fmt.Println("OPTIONS:")
		fmt.Println("    Options can be specified using either format:")
		fmt.Println("    -src        Single dash format")
		fmt.Println("    --src       Double dash format")
		fmt.Println("")
		fmt.Println("Run 'skelly help COMMAND' for command-specific help")
	}
}

func NewFlagCommandInvoker(
	downloader download.Downloader,
	processor *templates.Processor,
	ui view.UI,
) commands.Invoker {
	return &FlagCommandInvoker{
		downloader: downloader,
		processor:  processor,
		ui:         ui,
	}
}
