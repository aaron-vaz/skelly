package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aaron-vaz/skelly/internal/download"
	"github.com/aaron-vaz/skelly/internal/templates"
	"github.com/aaron-vaz/skelly/internal/view"
)

const (
	defaultDestination = "."
	downloadTimeout    = 5 * time.Minute
)

// InitOptions holds the parsed flag inputs for the init command.
type InitOptions struct {
	source      string
	destination string
}

// InitCommand encapsulates the logic required to download, process,
// and generate a new project from a specified template source.
type InitCommand struct {
	processor  *templates.Processor
	downloader download.Downloader
	ui         view.UI
	options    InitOptions
}

// Name returns the CLI identifier for this command.
func (c *InitCommand) Name() string {
	return "init"
}

// Description returns a short summary shown in the help output.
func (c *InitCommand) Description() string {
	return "Initialize a new project from a template"
}

// Init binds the required configuration flags to the InitCommand's options.
func (c *InitCommand) Init(flags *flag.FlagSet) {
	flags.StringVar(&c.options.source, "src", "", "URL to template that will be used to init the new project (required, may also be given as the first argument)")
	flags.StringVar(&c.options.destination, "dst", defaultDestination, "Destination dir where the project will be initialised to")
}

// Run downloads and renders the template in a scratch directory, and only
// once that has fully succeeded writes the result into the destination.
// A failed download or render therefore never touches the destination.
func (c *InitCommand) Run(ctx context.Context, args []string) error {
	if err := c.resolveOptions(args); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}
	dest := c.options.destination

	proceed, overwrite, err := c.confirmDestination(ctx, dest)
	if err != nil {
		return err
	}
	if !proceed {
		return c.ui.RenderInfo("Exiting....")
	}

	workDir, err := os.MkdirTemp("", "skelly-*")
	if err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()
	templateDir := filepath.Join(workDir, "template")

	downloadCtx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	if err := c.downloader.Get(downloadCtx, c.options.source, templateDir); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	config, err := c.processor.CreateTemplate(templateDir)
	if err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	// Without a config there is nothing to render, so the files are used as-is
	if config == nil {
		_ = c.ui.RenderInfo(fmt.Sprintf("No %s file found in template, copying files without rendering", templates.TemplateConfigName))
	} else {
		if err := c.ui.RenderInputs(ctx, config.Inputs); err != nil {
			return fmt.Errorf("failed to collect inputs: %w", err)
		}

		if err := c.processor.ApplyTemplate(*config, templateDir); err != nil {
			return fmt.Errorf("failed to render template: %w", err)
		}
	}

	// From here on the destination is being replaced, so finish even if
	// interrupted: stopping halfway would leave it neither old nor new.
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = context.WithoutCancel(ctx)

	if overwrite {
		if err := clearDir(dest); err != nil {
			return fmt.Errorf("failed to clean destination directory: %w", err)
		}
	}

	if err := download.CopyDir(ctx, templateDir, dest); err != nil {
		return fmt.Errorf("failed to write project to %s: %w", dest, err)
	}

	return c.ui.RenderInfo(fmt.Sprintf("Project created in %s", dest))
}

// resolveOptions allows the source to be given positionally, as in
// "skelly init github.com/user/template", and validates the result.
func (c *InitCommand) resolveOptions(args []string) error {
	if c.options.source == "" && len(args) > 0 {
		c.options.source, args = args[0], args[1:]
	}

	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, " "))
	}

	if c.options.source == "" {
		return errors.New("source is required")
	}

	if c.options.destination == "" {
		return errors.New("destination path is required")
	}

	return nil
}

// confirmDestination checks whether dest already has content, and if so asks
// the user whether it may be replaced. proceed is false if the user declined,
// and overwrite reports whether the existing contents need clearing first.
func (c *InitCommand) confirmDestination(ctx context.Context, dest string) (proceed bool, overwrite bool, err error) {
	empty, err := isEmptyDir(dest)
	if err != nil {
		return false, false, fmt.Errorf("failed to inspect destination: %w", err)
	}
	if empty {
		return true, false, nil
	}

	if err := checkSafeToClear(dest); err != nil {
		return false, false, err
	}

	answer, err := c.ui.RenderQuestion(ctx, fmt.Sprintf("Destination %s is not empty. Delete its contents and replace them with the new project?", dest), []string{"y", "n"})
	if err != nil {
		return false, false, fmt.Errorf("failed to get user input: %w", err)
	}

	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		return false, false, nil
	}

	return true, true, nil
}

// isEmptyDir reports whether dir is missing or an empty directory.
func isEmptyDir(dir string) (bool, error) {
	f, err := os.Open(dir)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", dir)
	}

	if _, err := f.Readdirnames(1); errors.Is(err, io.EOF) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}

// checkSafeToClear refuses to wipe directories where that is almost certainly
// a mistake, such as running "skelly init <src>" from the home directory.
func checkSafeToClear(dest string) error {
	abs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	protected := []string{filepath.VolumeName(abs) + string(filepath.Separator)}
	if home, err := os.UserHomeDir(); err == nil {
		protected = append(protected, filepath.Clean(home))
	}

	for _, p := range protected {
		if abs == p {
			return fmt.Errorf("refusing to overwrite %s, choose an empty or new directory with -dst", abs)
		}
	}
	return nil
}

// clearDir removes everything inside dir but keeps dir itself, so it is safe
// to use on the current working directory.
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// NewInitCommand constructs a new InitCommand initialized with the necessary
// template parsing, downloading, and UI rendering components.
func NewInitCommand(
	processor *templates.Processor,
	downloader download.Downloader,
	ui view.UI,
) Command {
	return &InitCommand{
		processor:  processor,
		downloader: downloader,
		ui:         ui,
		options:    InitOptions{},
	}
}
