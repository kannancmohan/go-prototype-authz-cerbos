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
.
├── flake.nix          # Nix flake definition
├── flake.lock         # Locked dependencies
├── go.mod            # Go module file
├── main.go           # Main application
├── cerbos/           # Cerbos policy files
├── README.md         # This file
└── LICENSE           # Project license
```

## Troubleshooting

### Hash Mismatch Errors

If you encounter hash mismatch errors when downloading Node.js or Go, you need to update the SHA256 hashes in `flake.nix` with the correct values from the official distribution sites.