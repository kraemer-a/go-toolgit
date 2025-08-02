# GitHub & Bitbucket Replace Tool - CLI Documentation

This directory contains comprehensive documentation for the GitHub & Bitbucket Replace Tool CLI commands.

## Overview

The GitHub & Bitbucket Replace Tool is a powerful CLI and GUI application for performing automated string replacements across multiple repositories on GitHub and Bitbucket Server.

## Quick Start

```bash
# Validate your configuration
go-toolgit validate

# List accessible repositories
go-toolgit list

# Perform replacements with dry-run
go-toolgit replace --dry-run --replacements "oldString=newString"

# Apply actual changes
go-toolgit replace --replacements "oldString=newString"

# Launch GUI interface
go-toolgit --gui
```

## Available Commands

- **[go-toolgit](go-toolgit.md)** - Main command and global options
- **[go-toolgit_validate](go-toolgit_validate.md)** - Validate configuration and access
- **[go-toolgit_list](go-toolgit_list.md)** - List accessible repositories
- **[go-toolgit_search](go-toolgit_search.md)** - Search for repositories
- **[go-toolgit_replace](go-toolgit_replace.md)** - Replace strings across repositories
- **[go-toolgit_replace-search](go-toolgit_replace-search.md)** - Replace strings in searched repositories
- **[go-toolgit_bitbucket-replace](go-toolgit_bitbucket-replace.md)** - Replace strings in Bitbucket repositories
- **[go-toolgit_docs](go-toolgit_docs.md)** - Generate this documentation

## Configuration

The tool supports multiple configuration methods:

1. **Configuration file**: `~/.go-toolgit/config.yaml`
2. **Environment variables**: `GITHUB_REPLACE_*` prefix
3. **Command-line flags**: See individual command documentation

## Examples

### GitHub Examples

```bash
# Basic replacement
go-toolgit replace --org mycompany --team backend --replacements "oldAPI=newAPI"

# Multiple replacements with file patterns
go-toolgit replace --replacements "jQuery=React,$.ajax=fetch" --include "*.js,*.jsx"

# Enterprise GitHub
go-toolgit replace --github-url "https://github.company.com" --token "$TOKEN" --replacements "v1=v2"
```

### Bitbucket Examples

```bash
# Basic Bitbucket replacement
go-toolgit bitbucket-replace --bitbucket-url "https://bitbucket.company.com" \
  --bitbucket-username "user" --bitbucket-password "$TOKEN" \
  --bitbucket-project "PROJ" --replacements "oldFunction=newFunction"
```

### Search Examples

```bash
# Search repositories by owner
go-toolgit search --owner myorg

# Search and replace in one command
go-toolgit replace-search --owner myorg --language go --replacements "oldAPI=newAPI"
```

## Support

For issues and feature requests, please visit: https://github.com/your-org/go-toolgit

