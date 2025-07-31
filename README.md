# proj - New projects made easy

A CLI tool that helps you initialize new projects from templates. It downloads templates from any source supported by [go-getter](https://github.com/hashicorp/go-getter) and processes them using a simple templating system.

## Installation

### From Source
```bash
go install github.com/aaron-vaz/proj/cmd/proj@latest
```

## Usage

Initialize a new project from a template:
```bash
# Basic usage
proj init --src https://github.com/user/template

# Specify custom destination
proj init --src https://github.com/user/template --dst ./my-project
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
