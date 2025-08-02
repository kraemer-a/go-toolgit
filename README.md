# GitHub & Bitbucket Replace Tool

A powerful CLI and GUI tool for performing automated string replacements across multiple repositories on **GitHub** and **Bitbucket Server**. Built with Go, featuring both command-line interface (Cobra) and modern web-based GUI (Wails).

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Features

- 🔄 **Multi-Repository Processing**: Replace strings across GitHub organizations/teams and Bitbucket projects
- 🔍 **Repository Search**: Find repositories beyond org/team constraints using GitHub search API
- 🏢 **Multi-Platform Support**: Works with GitHub.com, GitHub Enterprise, and Bitbucket Server
- 🎯 **Dual Interface**: Professional CLI with interactive spinners + modern web-based GUI
- 🔎 **Pattern Matching**: Support for regex and literal string replacements
- 📁 **File Filtering**: Include/exclude patterns for precise file targeting
- 🧪 **Dry-Run Mode**: Preview changes before execution
- 🔀 **Automated PRs**: Creates pull requests with your changes automatically
- ⚡ **Real-Time Progress**: Interactive spinners (CLI) and progress bars (GUI)
- 🛡️ **Error Handling**: Comprehensive validation and recovery mechanisms

## Installation

### Option 1: Build from Source
```bash
git clone https://github.com/your-org/go-toolgit.git
cd go-toolgit
go build -o go-toolgit
```

### Option 2: Build with Wails (GUI Support)
```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build the application
wails build
```

## Quick Start

### 1. Configuration

Create a configuration file at `~/.go-toolgit/config.yaml`:

#### For GitHub:
```yaml
provider: "github"  # Optional, defaults to github
github:
  base_url: "https://api.github.com"  # or your GitHub Enterprise URL
  token: "ghp_your_personal_access_token"
  organization: "your-org"
  team: "your-team"

processing:
  include_patterns:
    - "*.go"
    - "*.java"
    - "*.js"
    - "*.py"
  exclude_patterns:
    - "vendor/*"
    - "node_modules/*"
    - "*.min.js"

pull_request:
  title_template: "chore: automated string replacement"
  body_template: "Automated replacement performed by GitHub Replace Tool"
  branch_prefix: "auto-replace"
```

#### For Bitbucket Server:
```yaml
provider: "bitbucket"
bitbucket:
  base_url: "https://bitbucket.company.com"  # Your Bitbucket Server URL
  username: "your-username"
  password: "your-personal-access-token"  # Or password
  project: "PROJ"  # Bitbucket project key

processing:
  include_patterns:
    - "*.go"
    - "*.java"
    - "*.js"
    - "*.py"
  exclude_patterns:
    - "vendor/*"
    - "node_modules/*"
    - "*.min.js"

pull_request:
  title_template: "chore: automated string replacement"
  body_template: "Automated replacement performed by GitHub Replace Tool"
  branch_prefix: "auto-replace"
```

### 2. Validate Setup

```bash
# Test your configuration and GitHub access
./go-toolgit validate
```

### 3. List Repositories

```bash
# See which repositories will be affected
./go-toolgit list
```

### 4. Perform Replacements

```bash
# Replace strings across all team repositories
./go-toolgit replace --replacements "oldString=newString,deprecatedAPI=newAPI"

# Preview changes with dry-run
./go-toolgit replace --dry-run --replacements "oldString=newString"
```

### 5. Launch GUI

```bash
# Start the graphical interface
./go-toolgit --gui
```

## CLI Usage

### Commands

| Command | Description |
|---------|-------------|
| `validate` | Validate configuration and provider access (GitHub/Bitbucket) |
| `list` | List repositories accessible to the team/project |
| `search` | Search for repositories on GitHub using various criteria |
| `replace` | Perform string replacements across GitHub repositories |
| `bitbucket-replace` | Perform string replacements across Bitbucket repositories |
| `replace-search` | Replace strings in repositories found by search |
| `help` | Show help for any command |

### Global Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--config` | Configuration file path | `--config ./my-config.yaml` |
| `--provider` | Git provider (github, bitbucket) | `--provider bitbucket` |
| `--github-url` | GitHub base URL | `--github-url https://github.company.com` |
| `--token` | GitHub personal access token | `--token ghp_xxx` |
| `--org` | Organization name | `--org my-company` |
| `--team` | Team name | `--team backend-team` |
| `--bitbucket-url` | Bitbucket Server base URL | `--bitbucket-url https://bitbucket.company.com` |
| `--bitbucket-username` | Bitbucket username | `--bitbucket-username myuser` |
| `--bitbucket-password` | Bitbucket password/token | `--bitbucket-password mytoken` |
| `--bitbucket-project` | Bitbucket project key | `--bitbucket-project PROJ` |
| `--gui` | Launch GUI interface | `--gui` |

### Replace Command Options

| Flag | Description | Example |
|------|-------------|---------|
| `--replacements` | String pairs to replace | `--replacements "old=new,foo=bar"` |
| `--include` | File patterns to include | `--include "*.go,*.java"` |
| `--exclude` | File patterns to exclude | `--exclude "vendor/*"` |
| `--dry-run` | Preview changes only | `--dry-run` |
| `--pr-title` | Custom PR title | `--pr-title "Update API calls"` |
| `--max-workers` | Concurrent workers | `--max-workers 8` |

### Search Command Options

| Flag | Description | Example |
|------|-------------|---------|
| `--query` | Search keywords | `--query "microservice"` |
| `--owner` | Repository owner | `--owner gurke` |
| `--language` | Programming language | `--language go` |
| `--stars` | Star count filter | `--stars ">100"` or `--stars "10..50"` |
| `--size` | Repository size in KB | `--size ">1000"` |
| `--include-forks` | Include forked repositories | `--include-forks` |
| `--include-archived` | Include archived repositories | `--include-archived` |
| `--sort` | Sort by: stars, forks, updated | `--sort stars` |
| `--order` | Sort order: asc, desc | `--order desc` |
| `--limit` | Maximum results | `--limit 50` |
| `--output` | Output format: table, json | `--output json` |

### Replace-Search Command Options

Combines all search options above with replacement options:

| Flag | Description | Example |
|------|-------------|---------|
| `--max-repos` | Max repositories to process | `--max-repos 25` |
| All search flags | Same as search command | See search options above |
| All replace flags | Same as replace command | See replace options above |

## GUI Interface

The web-based GUI provides an intuitive interface for:

- **Configuration Management**: Easy setup of GitHub connection
- **Repository Selection**: Visual selection of target repositories
- **Rule Builder**: Interactive creation of replacement rules
- **Progress Tracking**: Real-time processing status
- **Results Display**: Detailed results with PR links

Launch with: `./go-toolgit --gui`

## Examples

### Basic String Replacement
```bash
./go-toolgit replace \
  --org mycompany \
  --team backend \
  --replacements "oldFunction=newFunction"
```

### Multiple Replacements with Patterns
```bash
./go-toolgit replace \
  --replacements "jQuery=React,$.ajax=fetch" \
  --include "*.js,*.jsx" \
  --exclude "node_modules/*,*.min.js" \
  --pr-title "feat: migrate from jQuery to modern APIs"
```

### Advanced GitHub Configuration
```bash
./go-toolgit replace \
  --github-url "https://github.company.com" \
  --token "$GITHUB_TOKEN" \
  --org "engineering" \
  --team "platform" \
  --replacements "v1/api=v2/api" \
  --include "*.go,*.yaml" \
  --branch-prefix "api-migration" \
  --max-workers 4
```

### Bitbucket Server Examples
```bash
# Basic Bitbucket replacement
./go-toolgit bitbucket-replace \
  --bitbucket-url "https://bitbucket.company.com" \
  --bitbucket-username "myuser" \
  --bitbucket-password "$BITBUCKET_TOKEN" \
  --bitbucket-project "PROJ" \
  --replacements "oldFunction=newFunction"

# Advanced Bitbucket replacement with filtering
./go-toolgit bitbucket-replace \
  --bitbucket-url "https://bitbucket.company.com" \
  --bitbucket-username "myuser" \
  --bitbucket-password "$BITBUCKET_TOKEN" \
  --bitbucket-project "BACKEND" \
  --replacements "jQuery=React,$.ajax=fetch" \
  --include "*.js,*.jsx" \
  --exclude "node_modules/*,*.min.js" \
  --pr-title "feat: migrate from jQuery to modern APIs" \
  --dry-run
```

### Repository Search Examples

```bash
# Search repositories by owner/organization
./go-toolgit search --owner gurke

# Search Go repositories with high stars
./go-toolgit search --language go --stars ">100"

# Complex search with multiple criteria
./go-toolgit search \
  --query "microservice" \
  --language go \
  --owner myorg \
  --stars "10..100" \
  --sort stars \
  --order desc

# Search and replace in one command
./go-toolgit replace-search \
  --owner gurke \
  --language go \
  --replacements "oldAPI=newAPI,fmt.Print=log.Info" \
  --dry-run

# Search private repositories (requires appropriate token permissions)
./go-toolgit search \
  --owner mycompany \
  --include-archived \
  --max-repos 25
```

## Configuration

### Environment Variables

All configuration options can be set via environment variables with the `GITHUB_REPLACE_` prefix:

#### GitHub Environment Variables:
```bash
export GITHUB_REPLACE_PROVIDER="github"
export GITHUB_REPLACE_GITHUB_TOKEN="ghp_your_token"
export GITHUB_REPLACE_GITHUB_BASE_URL="https://github.company.com"
export GITHUB_REPLACE_GITHUB_ORGANIZATION="my-org"
export GITHUB_REPLACE_GITHUB_TEAM="my-team"
```

#### Bitbucket Environment Variables:
```bash
export GITHUB_REPLACE_PROVIDER="bitbucket"
export GITHUB_REPLACE_BITBUCKET_BASE_URL="https://bitbucket.company.com"
export GITHUB_REPLACE_BITBUCKET_USERNAME="myuser"
export GITHUB_REPLACE_BITBUCKET_PASSWORD="mytoken"
export GITHUB_REPLACE_BITBUCKET_PROJECT="PROJ"
```

### Configuration Hierarchy

1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

## Development

### Prerequisites

- Go 1.24+
- Node.js 16+ (for GUI development)
- Git

### Building

```bash
# CLI only
go build -o go-toolgit

# With GUI support
wails build

# Development mode with live reload
wails dev
```

### Project Structure

```
go-toolgit/
├── cmd/cli/              # Cobra CLI commands
├── internal/core/        # Shared business logic
│   ├── config/          # Configuration management
│   ├── github/          # GitHub API client
│   ├── bitbucket/       # Bitbucket Server API client
│   ├── processor/       # String replacement engine
│   ├── git/             # Git operations
│   └── utils/           # Utilities (logging, errors, spinners)
├── internal/gui/         # GUI-specific handlers
├── frontend/             # Web frontend (HTML/CSS/JS)
└── main.go              # Application entry point
```

### Testing

```bash
go test ./...
go test -v ./internal/core/...
go test -cover ./...
```

## Authentication Setup

### GitHub Token Setup

1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate a new token with these permissions:
   - `repo` (Full control of private repositories)
   - `read:org` (Read org and team membership)
   - `user:email` (Access user email addresses)

For GitHub Enterprise, ensure the token has equivalent permissions.

### Bitbucket Server Authentication

1. **Personal Access Token (Recommended)**:
   - Go to Bitbucket Server → Personal settings → Personal access tokens
   - Create a token with `REPO_READ`, `REPO_WRITE`, and `PROJECT_READ` permissions
   - Use the token as the password in configuration

2. **Username/Password**:
   - Use your regular Bitbucket Server username and password
   - Less secure than personal access tokens

3. **Required Permissions**:
   - Read access to the target project
   - Write access to repositories for creating branches
   - Permission to create pull requests

## Troubleshooting

### Common Issues

**Configuration validation failed**: Check that all required fields are set in your config file or environment variables.

**GitHub access validation failed**: Verify your token has the correct permissions and can access the specified organization and team.

**Failed to list team repositories**: Ensure the team exists and your token has access to it.

**Bitbucket access validation failed**: Verify your Bitbucket Server URL, username, and password/token are correct.

**Failed to list project repositories**: Ensure the Bitbucket project key exists and your user has read access to it.

**Pull request creation failed**: Check that you have write permissions to the repository and the branch names are valid.

### Debug Mode

Enable debug logging for detailed information:

```bash
./go-toolgit --log-level debug validate
```

## Security Considerations

- Store tokens securely (environment variables, not in code)
- Use minimal required permissions for GitHub tokens
- Review changes in dry-run mode before execution
- Consider using branch protection rules

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Wails](https://wails.io/) - GUI framework
- [yacspin](https://github.com/theckman/yacspin) - CLI spinners
- [Viper](https://github.com/spf13/viper) - Configuration management
