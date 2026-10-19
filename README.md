# skelly

<p align="center">
  <b>The bare bones of your next project.</b>
</p>

<p align="center">
  <a href="https://github.com/aaron-vaz/skelly/releases/latest"><img src="https://img.shields.io/github/v/release/aaron-vaz/skelly" alt="Latest Release"></a>
  <a href="https://goreportcard.com/report/github.com/aaron-vaz/skelly"><img src="https://goreportcard.com/badge/github.com/aaron-vaz/skelly" alt="Go Report Card"></a>
  <a href="/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

`skelly` is a command-line tool designed to streamline the creation of new software projects. Stop manually copying boilerplate files or cloning and cleaning up old repositories. With `skelly`, you can instantly scaffold a new project from any template, substituting variables to customize it on the fly.

It leverages the power of [go-getter](https://github.com/hashicorp/go-getter) to fetch templates from virtually anywhere including GitHub, GitLab, Bitbucket, or even a simple zip file over HTTP. The simple templating system, powered by Go's native `text/template` engine, makes creating your own templates a breeze.

## Motivation

This project was born from two goals: a desire to learn how to build a production-ready CLI application in Go, and the need to solve a recurring personal problem. At work, I saw the power of tools like [Backstage](https://backstage.io/) and [Cookiecutter](https://github.com/cookiecutter/cookiecutter) in streamlining development workflows, which inspired me to tackle my own repetitive setup process for personal projects.

I found that whenever I started a new project—whether in Go, Java, or Kotlin—I would spend a significant amount of time copying and pasting boilerplate to fit my preferred structure. I considered using an existing tool like [Cookiecutter](https://github.com/cookiecutter/cookiecutter), but ultimately decided to build something simple myself. This would not only solve my problem in a tailored way but also serve as a perfect learning exercise.

`skelly` is the result of that.

## Features

*   **Flexible Sources**: Fetch templates from Git, Mercurial, HTTP, S3, and more.
*   **Simple Templating**: Uses Go's built-in `text/template` for easy templating, Using a simple YAML configuration file to maintain the inputs for the templating.
*   **One Template, Many Variants**: `choices` limit what an input accepts, `exclude` drops the files a variant does not need, and `rename` moves files into place, all depending on the user's answers.
*   **Derived Values**: Input defaults and descriptions, `env` values, patterns and renames can all be templates that use other inputs, so a value is written once.
*   **Scriptable and AI-Native**: `skelly inspect` describes a template as JSON, and `init` takes `-set`, `-no-input`, `-yes` and `-json`, so scripts and AI agents can scaffold projects without answering prompts.
*   **Editor Support**: A JSON Schema for `.skelly.yml` gives autocomplete and inline errors.
*   **Cross-Platform**: A single, self-contained binary for Linux and macOS

## Installation

There are a couple of ways to install `skelly`.

### From Release (Recommended)

Download the pre-compiled binary for your operating system from the **latest release page**.

### From Source

Ensure you have Go (1.26 or later) installed on your system.

```bash
go install github.com/aaron-vaz/skelly/cmd/skelly@latest
```

## Usage

The primary command is `init`, which scaffolds a new project from a template into a destination directory.

```bash
# Initialize a new project from a GitHub repository template
skelly init --src https://github.com/user/template --dst ./my-new-app

# The source can also be given as a plain argument
skelly init github.com/user/template -dst ./my-new-app
```

The template is downloaded and rendered in a temporary directory first, so a failed download or render never touches the destination. If the destination already contains files, `skelly` asks before replacing them.

### Using skelly from scripts and AI agents

`skelly init` asks questions, which a script or an AI coding agent cannot answer. Everything it asks can be given up front instead, and nothing waits on stdin when `-no-input` is set:

```bash
# 1. Discover what a template needs: inputs (with defaults, choices and whether
#    they are required), raw patterns and exclusions, as JSON
skelly inspect github.com/user/template

# 2. Create the project without any prompt
skelly init github.com/user/template -dst ./my-new-app -no-input \
  -set project_name=my-new-app -set project_type=service
```

*   `-set name=value` answers an input and may be repeated. Unknown inputs and values outside an input's `choices` are rejected, and every problem is reported at once.
*   `-no-input` never prompts. Inputs you did not `-set` take their defaults, and it is an error if one has none (the error names them), or if the destination is not empty.
*   `-yes` replaces the contents of a destination that is not empty. It still refuses to wipe a home or root directory.
*   `-json` prints the outcome as a single JSON object (`source`, `destination` and the `inputs` used) and implies `-no-input`. Errors go to stderr with a non-zero exit code.
*   `skelly inspect` downloads the template to a scratch directory and prints JSON only. A templated default or description is shown as written, with `dependsOn` naming the inputs it is built from, since its final text depends on the answers. It never renders or writes anything else, so it is safe to run before deciding what to pass to `init`.
*   Template authors get editor help for `.skelly.yml` from [`skelly.schema.json`](skelly.schema.json).

### Registering template sources

Templates you use often can be registered under a short alias, so you don't have to type the full source every time:

```bash
# Register a source under an alias (local paths are stored as absolute paths)
skelly sources add go github.com/user/go-template

# Then pass the alias to init instead of the source
skelly init go -dst ./my-new-app

# List and remove registered sources
skelly sources list
skelly sources remove go

# Replace the source of an existing alias
skelly sources add -force go github.com/user/other-go-template
```

Aliases may contain letters, digits, `-` and `_`, so they never look like a URL or a path. If a local directory has the same name as an alias, pass it as `./name` to use the directory instead.

Registered sources are stored in a YAML config file, `$XDG_CONFIG_HOME/skelly/config.yml` (`~/.config/skelly/config.yml` by default). Set `SKELLY_CONFIG` to use a different file.

```yaml
# ~/.config/skelly/config.yml
sources:
  go:
    url: github.com/user/go-template
```

### Writing templates

A template is any directory (or repository) containing a `.skelly.yml` file. File contents **and** file/directory names are rendered with Go's [`text/template`](https://pkg.go.dev/text/template):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/aaron-vaz/skelly/main/skelly.schema.json
# .skelly.yml
name: my-template
description: A Go service skeleton
inputs:
  project_name:
    description: Name of the project
    default: my-app
  author:
    description: Author's name   # no default, so the input is required
  project_type:
    description: What kind of project to generate
    default: cli
    choices: [cli, service]      # any other answer is rejected and asked again
  module:
    description: Go module path of {{.Inputs.project_name.Value}}
    default: github.com/{{.Inputs.author.Value}}/{{.Inputs.project_name.Value}}   # a template: asked after the inputs it uses
# Values for templates to use as {{.Env.image}}. A string is itself a template that can use the inputs.
env:
  image: ghcr.io/{{.Inputs.author.Value}}/{{.Inputs.project_name.Value}}
# Paths left out of the generated project when the condition renders as true, or always without one.
exclude:
  - paths: [internal/server, Dockerfile]   # several paths can share one condition
    when: '{{eq .Inputs.project_type.Value "cli"}}'
  - path: "**/*_test.go"                    # ** matches across directories
    when: "false"
  - path: TEMPLATE.md                       # without a when, always excluded: the template's own files
# Paths moved once the project is rendered, so that files meant for generated projects
# can sit under an inert name in the template's own repository (see the notes below).
rename:
  - from: github
    to: .github
# Files copied without rendering their names or contents, e.g. files that use {{ }} themselves.
# A pattern can match a file's path, its name, or any parent directory.
raw:
  - github        # the template's own name, before the rename above
  - "*.tpl"
```

```text
{{.Inputs.project_name.Value}}/main.go   ->  my-app/main.go
// Author: {{.Inputs.author.Value}}       ->  // Author: Jane Doe
```

Notes:

*   Referencing an input that is not defined is an error, rather than silently rendering `<no value>`.
*   `.skelly.yml` is validated up front: input names must not be blank, defaults must be a string, number or boolean, and `raw` and `exclude` patterns must be valid globs, and `rename` paths must stay inside the project.
*   `exclude` lets one template produce several variants. `path` (or `paths`, for several globs sharing one condition) is matched like `raw` patterns (a file's path, its name, or any parent directory) and `when` is rendered as a template that must produce `true` or `false` (leave it out to always exclude, as for the template's own files). Excluded paths are removed before anything is rendered, and `exclude` also applies to `raw` files, so a raw CI workflow can still be left out conditionally.
*   Globs use [doublestar](https://github.com/bmatcuk/doublestar) syntax for `raw` and `exclude`: `*` and `?` stay within a directory, `**` matches across directories (`docs/**/*.md`, `**/*_test.go`), and `{a,b}` matches alternatives.
*   Because `raw` and `exclude` patterns are doublestar globs and templates, `{` and `}` mean alternatives (escape a literal brace with `\`) and `{{` starts a template. Patterns written for earlier versions that contain braces need that escape.
*   A template can never put a `.git` directory (or file) in the generated project: any `.git` in the template, at any depth, is not copied, and a rendered file or directory name, a `rename`, or a symlink that would bring one back is an error, since git runs the hooks inside it. Case, trailing dots and spaces, invisible characters and Windows short names such as `GIT~1` count as `.git`.
*   `exclude` and `rename` entries only accept their own keys, and a written but empty `path` or `paths` is an error. Without a `when` an entry always applies, so a misspelt `when` has to fail and not be ignored.
*   `-set` answers and typed answers follow the type of the input's default, so `-set docker=false` is the boolean `false` that `{{if .Inputs.docker.Value}}` sees, not the text "false". A value of the wrong type (`-set count=many` for a default of `3`, or `NaN` for a number) is rejected.
*   `skelly.schema.json` is a JSON Schema for `.skelly.yml`. Add the `# yaml-language-server: $schema=...` line shown above and editors with the YAML language server get autocomplete, docs and inline errors. It covers structure only; semantic checks, such as a default being one of its choices, still happen when the template is loaded.
*   Values in `.skelly.yml` can be templates too, so a value is written once and derived from the rest. An input's `description`, `default` and `choices` can use other inputs (`{{.Inputs.name.Value}}`); the input is then asked after the ones it uses, and skelly rejects an unknown input or a cycle between inputs when it loads the template. `env` string values can use the inputs but not each other. `raw` and `exclude` patterns, `exclude` and `rename` conditions (`when`) and `rename` paths (`from`, `to`) are rendered as well. Inputs cannot use `.Env`, which is derived from them, and a template must name an input as `.Inputs.name.Value` for skelly to see the dependency. The template's own `name`, `description` and `renderer` stay as written.
*   `rename` moves a path after everything has been rendered, in the order listed, and creates missing parent directories; `from` names the path as it was rendered, and `when` skips the rename when it renders as `false`, so variants can ship `Makefile.cli` and `Makefile.service` and rename the right one. Use it for files that would act on the template's own repository if they kept their real name: ship CI workflows and `dependabot.yml` under `github/` and rename that to `.github`, so GitHub only runs them in generated projects, and the same for `gitignore` to `.gitignore`. `raw` and `exclude` match the template's own file names, before anything is rendered or renamed, so use `github/workflows/ci.yml` there. A missing source, an existing target or a path outside the project is an error.
*   `choices` restricts an input to a fixed list of values; a `default` must be one of them.
*   A rendered name may contain `/` to create nested directories (handy for Java packages), but can never point outside the project.
*   Binary files and symlinks are copied without rendering; the template's `.git` directory and `.skelly.yml` are not copied at all.

Other commands:
```bash
# Show help
skelly help
skelly help init

# Show a template's inputs and settings as JSON
skelly inspect github.com/user/template

# Show version
skelly version
```

## Development

### Prerequisites
- Go 1.26 or later

### Building from source
```bash
# Clone the repository
git clone https://github.com/aaron-vaz/skelly.git
cd skelly

# Install dependencies
make deps

# Build, test and lint
make all

# Just build the binary
make build

# Run tests
make test

# Run linter
make lint
