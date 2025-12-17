# Common Nix Commands

## Flake Commands

```bash
# Enter development shell
nix develop

# Check flake structure and metadata
nix flake metadata

# Update flake inputs (e.g., nixpkgs)
nix flake update

# Show what packages a flake provides
nix flake show

# Build a specific package from the flake
nix build .#<package-name>
```

## Nix Store Management

```bash
# Show disk usage of nix store
nix store du

# Optimize store (remove duplicate files)
nix store optimise

# Garbage collection - remove unused packages
nix-store --gc

# Garbage collection with specific conditions
nix-store --gc --delete-older-than 30d  # Delete generations older than 30 days
```

## Nix Profile Management

```bash
# List installed packages in profile
nix profile list

# Install a package to profile
nix profile install nixpkgs#<package-name>

# Remove a package from profile
nix profile remove <index>

# Upgrade packages in profile
nix profile upgrade
```

## Finding Packages

```bash
# Search for packages in nixpkgs
nix search nixpkgs <search-term>

# Example: Search for Go packages
nix search nixpkgs go
```

## System Information

```bash
# Show nix configuration
nix show-config

# Show system information
nix doctor