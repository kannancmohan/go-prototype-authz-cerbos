# Go Prototype with Cerbos Authorization

This project provides a development environment for a Go prototype with Cerbos 

## Development Environment Setup

### Prerequisites

- [Nix package manager](https://nixos.org/download.html) installed
- Git (for version control)

### Entering the Development Environment

To enter the development shell with all required tools:

```bash
nix develop
```

This will:
- Download and install Go 1.25.2
- Install Node.js 18.20.4
- Set up Go language server (gopls) and debugger (delve)
- Display the versions of installed tools

### Development Tools

This project uses several tools to maintain code quality and development workflow:

#### Git Hooks with Lefthook
[Lefthook](https://github.com/evilmartians/lefthook) is a fast and powerful Git hooks manager that automates quality checks:

- **Pre-commit hooks**: Automatically runs linting, formatting, tests, and secret detection
- **Commit message validation**: Ensures commit messages follow conventional commit standards
- **Pre-push hooks**: Runs tests and security checks before pushing

The configuration is defined in [`.config/lefthook.yml`](.config/lefthook.yml).

#### Commit Message Standards with Commitlint
[Commitlint](https://commitlint.js.org/) checks if your commit messages meet the conventional commit format:

- Enforces conventional commit types (feat, fix, docs, etc.)
- Validates commit message structure and length
- Configuration in [`.config/commitlint.config.mjs`](.config/commitlint.config.mjs)

#### Code Quality Tools
- **golangci-lint**: Comprehensive Go linting (see [Code Quality and Linting](#code-quality-and-linting) section)
- **gitleaks**: Secret detection to prevent accidental credential commits

## Running the Application

### Manual Start

To start the application manually:

```bash
# From the project root directory
go run cmd/main.go
```

### Build and Run

To build the application first and then run it:

```bash
# Build the application
go build -o bin/app cmd/main.go

# Run the built application
./bin/app
```

## Code Quality and Linting

### Manual Linting with golangci-lint

This project uses [golangci-lint](https://golangci-lint.run/) for comprehensive code quality checks. The configuration is defined in [`.golangci.yml`](.golangci.yml).

#### Run Linter (Check Only)

To run the linter and see all issues without fixing them:

```bash
golangci-lint run
```

#### Run Linter with Auto-fix

To run the linter and automatically fix issues where possible:

```bash
golangci-lint run --fix
```

#### Run Specific Linters

To run only specific linters (e.g., only security checks):

```bash
golangci-lint run --enable-only gosec
```

#### Run Formatters Only

```bash
golangci-lint fmt
```

## Spec-Driven Development

This project uses **Spec-Driven Development** with [GitHub Spec Kit](https://github.com/github/spec-kit) to define and manage project specifications.

### Installing Spec Kit

Spec Kit is installed and run using `uvx` (uv's tool runner), which automatically manages the tool in an isolated environment.

### Initialize Project Specifications

To initialize or update the project specifications:

```bash
# Run from the project root directory
uvx --from git+https://github.com/github/spec-kit.git specify init . --force
```

Running the initialization command will:

**Create a `.specify/` directory** in your project root containing:
   - `memory/` - Project memory and constitution files
   - `scripts/` - Automation scripts for development workflow
   - `templates/` - Templates for plans, specs, tasks, and other documents

## Project Structure

```

```

## Troubleshooting

### Hash Mismatch Errors

If you encounter hash mismatch errors when downloading Node.js or Go, you need to update the SHA256 hashes in `flake.nix` with the correct values from the official distribution sites.