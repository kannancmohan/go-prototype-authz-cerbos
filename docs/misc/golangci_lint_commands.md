# Common golangci-lint Commands

This project uses [golangci-lint](https://golangci-lint.run/) for comprehensive code quality checks. The configuration is defined in [`.golangci.yml`](../.golangci.yml).

## Basic Linting

```bash
# Run all linters on the entire project
golangci-lint run

# Run linters on a specific package or directory
golangci-lint run ./pkg/...
golangci-lint run ./cmd/...

# Run linters on a single file
golangci-lint run pkg/logger/logger.go
```

## Auto-fix Issues

```bash
# Automatically fix issues where possible
golangci-lint run --fix

# Fix issues in a specific directory
golangci-lint run --fix ./pkg/...
```

## Selective Linting

```bash
# Run only specific linters (e.g., security checks)
golangci-lint run --enable-only gosec

# Run multiple specific linters
golangci-lint run --enable-only gosec,staticcheck,errcheck

# Disable specific linters
golangci-lint run --disable lll,funlen

# Run only specific linters with auto-fix
golangci-lint run --enable-only=nlreturn --fix
```

## Formatting

```bash
# Run only formatters (gofmt, gofumpt, gci, golines)
golangci-lint fmt
```

## Configuration and Information

```bash
# Show current linters configuration
golangci-lint linters

# Show formatters configuration
golangci-lint formatters

# Validate configuration file
golangci-lint config

# Display version
golangci-lint version
```

## Cache Management

```bash
# Clean the cache
golangci-lint cache clean

# Show cache status
golangci-lint cache status
```

## Usage Tips

- Use `--fix` flag to automatically fix many issues.
- Use `--enable-only` to focus on specific linting categories.
- The linter runs automatically in pre‑commit hooks via lefthook.
- For more options, run `golangci-lint --help` or `golangci-lint run --help`.