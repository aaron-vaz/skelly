# AGENTS.md

Guidelines for agentic coding agents working in this repository.

## Project Overview

`skelly` is a Go CLI tool for scaffolding new projects from templates. It uses go-getter to fetch templates and Go's text/template engine for templating.

- **Language**: Go 1.26
- **Binary**: `skelly`
- **Module**: `github.com/aaron-vaz/skelly`

## Using skelly yourself

Agents should never rely on prompts. `skelly inspect <src>` prints a template's inputs as JSON, and `skelly init <src> -dst <dir> -no-input -set name=value` creates a project without asking anything (add `-yes` to replace a non-empty destination, `-json` for a machine-readable result). See "Using skelly from scripts and AI agents" in the README.

## Build/Lint/Test Commands

### All-in-one
```bash
make all        # Runs lint, test, and build
```

### Building
```bash
make build      # Build binary for current platform -> build/skelly
make release    # Build for all platforms (linux/darwin, amd64/arm64)
make clean      # Remove build artifacts
```

### Testing
```bash
make test       # Run all tests with coverage -> build/coverage.out
go test -v -race ./...                    # Run all tests directly
go test -v -race -run TestName ./pkg/path # Run single test
go test -v -race -run TestName ./...      # Run single test (search all packages)
```

### Linting
```bash
make lint       # Run golangci-lint (requires installation)
golangci-lint run                        # Run directly
```

### Dependencies
```bash
make deps       # Install dependencies and golangci-lint
```

## Code Style Guidelines

### Package Structure

```
cmd/skelly/        # Main entry point
internal/          # Private packages (not importable externally)
  autocomplete/    # Completion of a command line, and the bash/zsh/fish scripts that ask for it
  cli/             # CLI parsing and command invocation
  commands/        # Command implementations (Group nests subcommands, e.g. "sources add"); init, inspect, sources, help, version
  config/          # User config file (registered template sources, settings)
  download/        # Template download functionality
  templates/       # Template processing and rendering
  view/            # User interface (I/O)
skelly.schema.json # JSON Schema for .skelly.yml (editor support, see below)
install.sh         # POSIX sh installer that verifies a release download (see below)
```

### Imports

Group imports with blank lines between:
1. Standard library
2. External/third-party packages
3. Local packages (github.com/aaron-vaz/skelly/...)

```go
import (
    "context"
    "errors"
    "fmt"

    "gopkg.in/yaml.v2"

    "github.com/aaron-vaz/skelly/internal/templates"
)
```

### Naming Conventions

- **Interfaces**: Single-word nouns (`Downloader`, `UI`, `Renderer`, `Command`)
- **Structs**: PascalCase (`InitCommand`, `Processor`, `StdUI`)
- **Methods**: PascalCase (exported), camelCase (private)
- **Private struct fields**: camelCase (`options`, `renderer`)
- **Constants**: PascalCase or UPPER_SNAKE_CASE (`TemplateConfigName`)
- **Acronyms**: Keep consistent casing (`StdUI`, not `StdUi`)

### Constructor Functions

Use `NewXxx()` pattern. Return interface types when appropriate:

```go
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
```

### Error Handling

- Return errors, don't panic (except in main)
- Wrap errors with context using `fmt.Errorf` and `%w`:
  ```go
  return fmt.Errorf("failed to download template: %w", err)
  ```
- Use `errors.Is()` for error comparison:
  ```go
  if errors.Is(err, os.ErrNotExist) {
      // handle not found
  }
  ```
- Main function handles error output to stderr

### Interfaces

- Keep interfaces small and focused
- Define interfaces where they are used
- Document all interface methods:

```go
type Downloader interface {
    // Get downloads a file from source to destination
    Get(ctx context.Context, source string, destination string) error
}
```

### Generics

Only reach for generics when a type parameter genuinely varies. Commands are a
plain interface because every command is configured through `*flag.FlagSet`:

```go
type Command interface {
    Name() string
    Description() string
    Init(flags *flag.FlagSet)
    Run(ctx context.Context, args []string) error
}
```

### Template Config (.skelly.yml)

The Go types in `internal/templates/definitions.go` and `Validate` are the source of truth for the config format. `skelly.schema.json` mirrors the structure for editors only, so any change to the config's shape (a new key, a new accepted form) must be made in both, and `internal/templates/schema_test.go` must keep passing: it fails if the schema and `Validate` disagree. Document new keys in the README as well.

Values in the config are templates where it says so in `definitions.go` (input `description`/`default`/`choices`, `env` values, `raw` and `exclude` patterns, `when`, `rename` paths). Inputs are asked in dependency order (`InputLayers`), and `Validate` rejects cycles and unknown references, so keep new templated fields in `inputTemplates` and `Validate`.

Template functions live in `templateFuncs` (`internal/templates/render_std.go`). Every place that parses a template must register them with `.Funcs(templateFuncs)`, or a call is reported as undefined when the config is loaded. Keep them pure string functions: templates are untrusted, so nothing that reads files, the environment or runs commands.

`raw` and `exclude` patterns use doublestar globs (`**` crosses directories). Keep new commands scriptable: no prompt without a way to answer it by flag, and anything meant for a program goes to stdout as JSON while errors go to stderr.

### Filesystem Safety

Templates are untrusted input. Any code that reads or writes inside a template
directory must go through an `os.Root` (see `internal/templates/processor.go`)
so symlinks and rendered names cannot escape it, and `init` must only touch the
destination after download and rendering have fully succeeded.

### Releases and install.sh

`make release` builds the four binaries and writes `build/bin/checksums.txt` (`shasum -a 256`, the format `sha256sum -c` reads). The release workflow attests the binaries and `checksums.txt` and uploads them all, and `install.sh` downloads a binary plus `checksums.txt` and refuses to install unless the hash matches. So keep the binary names `skelly_<os>_<arch>`: they are what the installer looks for.

`install.sh` is plain POSIX `sh`, and everything in it is inside `main`, which runs last, so a truncated download runs nothing. Keep it `shellcheck --shell=sh` clean (CI checks), validate anything that reaches a URL or a path, and fail closed: a missing or mismatched checksum is an error, never a warning. Its tests are in `cmd/skelly/install_integration_test.go`.

The `checksums.txt` of the releases up to v0.5.1 was added afterwards, by a one-off workflow (removed; see the history of `.github/workflows/backfill-checksums.yml`) that hashed the published binaries and compared them with the digests GitHub reports. Those releases therefore have no build attestation, and `install.sh --verify-provenance` refuses them.

### Shell completion

`skelly autocomplete <bash|zsh|fish>` prints a script for the user's startup file, and the script calls `skelly autocomplete complete -- <words>` on every Tab. All the logic is in `autocomplete.Complete` (`internal/autocomplete`), so keep the scripts thin and put new behaviour there, where it is unit tested. The commands, subcommands and flags are read from the real commands (`Init` and `Group`), so a new flag or command is completed without any change. What a flag value or a positional argument is completed with (a directory, a template alias or source) is in `completionHints` in `internal/commands/autocomplete_command.go`: add an entry for a command that takes one. A test fails if a hint names a flag that no longer exists. A completion must never fail loudly or print anything but candidates on stdout, because it is run for every keypress and its output is parsed. The shell tests are in `cmd/skelly/autocomplete_integration_test.go` and run each shell that is installed.

### File Organization

Within a file:
1. Package declaration
2. Imports
3. Package-level variables/constants
4. Types (interfaces before structs)
5. Struct types
6. Methods on structs
7. Helper functions
8. Constructor function at the bottom

### Comments

- Add package-level comments for documentation
- Document all exported types, functions, and interface methods
- Use minimal inline comments; prefer self-documenting code
- Comment "why" not "what"

### Testing Conventions

When writing tests:
- Place test files in the same package (`package foo` not `package foo_test`)
- Name test functions `TestFunctionName` or `TestFunctionName_Scenario`
- Use table-driven tests for multiple cases
- Run tests with `-race` flag
- **BDD Syntax**: Structure tests using BDD-style comments (`// Given`, `// When`, `// Then`, `// And`) to clearly delineate setup, execution, and validation phases.
- **Integration Tests**: Command-level E2E tests run the compiled binary and live in `cmd/skelly/`. Read `cmd/skelly/AGENTS.md` before adding or changing one; it covers the helpers, fixtures and conventions.


### Context Usage

Use context for operations that may timeout or be cancelled:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

### Code Formatting

- Use `gofmt` or `go fmt` for formatting
- Run `golangci-lint` before committing
- No unused imports or variables
- Prefer explicit error returns over ignoring them
