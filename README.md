# proj

<p align="center">
  <b>New projects made easy.</b>
</p>

<p align="center">
  <a href="https://github.com/aaron-vaz/proj/releases/latest"><img src="https://img.shields.io/github/v/release/aaron-vaz/proj" alt="Latest Release"></a>
  <a href="https://goreportcard.com/report/github.com/aaron-vaz/proj"><img src="https://goreportcard.com/badge/github.com/aaron-vaz/proj" alt="Go Report Card"></a>
  <a href="/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

`proj` is a command-line tool designed to streamline the creation of new software projects. Stop manually copying boilerplate files or cloning and cleaning up old repositories. With `proj`, you can instantly scaffold a new project from any template, substituting variables to customize it on the fly.

It leverages the power of [go-getter](https://github.com/hashicorp/go-getter) to fetch templates from virtually anywhere including GitHub, GitLab, Bitbucket, or even a simple zip file over HTTP. The simple templating system, powered by Go's native `text/template` engine, makes creating your own templates a breeze.

## Motivation

This project was born from two goals: a desire to learn how to build a production-ready CLI application in Go, and the need to solve a recurring personal problem. At work, I saw the power of tools like [Backstage](https://backstage.io/) and [Cookiecutter](https://github.com/cookiecutter/cookiecutter) in streamlining development workflows, which inspired me to tackle my own repetitive setup process for personal projects.

I found that whenever I started a new project—whether in Go, Java, or Kotlin—I would spend a significant amount of time copying and pasting boilerplate to fit my preferred structure. I considered using an existing tool like [Cookiecutter](https://github.com/cookiecutter/cookiecutter), but ultimately decided to build something simple myself. This would not only solve my problem in a tailored way but also serve as a perfect learning exercise.

`proj` is the result of that.

## Features

*   **Flexible Sources**: Fetch templates from Git, Mercurial, HTTP, S3, and more.
*   **Simple Templating**: Uses Go's built-in `text/template` for easy templating, Using a simple YAML configuration file to maintain the inputs for the templating.
*   **Cross-Platform**: A single, self-contained binary for Linux and macOS

## Installation

There are a couple of ways to install `proj`.

### From Release (Recommended)

Download the pre-compiled binary for your operating system from the **latest release page**.

### From Source

Ensure you have Go (1.24 or later) installed on your system.

```bash
go install github.com/aaron-vaz/proj/cmd/proj@latest
```

## Usage

The primary command is `init`, which scaffolds a new project from a template into a destination directory.

```bash
# Initialize a new project from a GitHub repository template
proj init --src https://github.com/user/template --dst ./my-new-app
```

Other commands:
```bash
# Show help
proj help
proj help init

# Show version
proj version
```

## Development

### Prerequisites
- Go 1.24 or later

### Building from source
```bash
# Clone the repository
git clone https://github.com/aaron-vaz/proj.git
cd proj

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
