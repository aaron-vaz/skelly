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

### Writing templates

A template is any directory (or repository) containing a `.skelly.yml` file. File contents **and** file/directory names are rendered with Go's [`text/template`](https://pkg.go.dev/text/template):

```yaml
# .skelly.yml
name: my-template
description: A Go service skeleton
inputs:
  project_name:
    description: Name of the project
    default: my-app
  author:
    description: Author's name   # no default, so the input is required
# Files whose contents are copied without rendering, e.g. files that use {{ }} themselves.
# A pattern can match a file's path, its name, or any parent directory.
raw:
  - .github
  - "*.tpl"
```

```text
{{.Inputs.project_name.Value}}/main.go   ->  my-app/main.go
// Author: {{.Inputs.author.Value}}       ->  // Author: Jane Doe
```

Notes:

*   Referencing an input that is not defined is an error, rather than silently rendering `<no value>`.
*   `.skelly.yml` is validated up front: input names must not be blank, defaults must be a string, number or boolean, and `raw` patterns must be valid globs.
*   A rendered name may contain `/` to create nested directories (handy for Java packages), but can never point outside the project.
*   Binary files and symlinks are copied without rendering; the template's `.git` directory and `.skelly.yml` are not copied at all.

Other commands:
```bash
# Show help
skelly help
skelly help init

# Show version
skelly version
```

## Development

### Prerequisites
- Go 1.24 or later

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
