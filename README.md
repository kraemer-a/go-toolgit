# GitHub & Bitbucket Replace Tool

A comprehensive CLI and GUI tool for **automated string replacements** and **repository migrations** across multiple repositories on **GitHub** and **Bitbucket Server**. Built with Go, featuring command-line interface (Cobra) and native desktop GUI (Fyne).

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Features

### 🔄 **String Replacement Engine**
- **Multi-Repository Processing**: Replace strings across GitHub organizations/teams and Bitbucket projects
- **Repository Search**: Find repositories beyond org/team constraints using GitHub search API
- **Pattern Matching**: Support for regex and literal string replacements
- **File Filtering**: Include/exclude patterns for precise file targeting
- **Dry-Run Mode**: Preview changes before execution
- **Automated PRs**: Creates pull requests with your changes automatically

### 🚚 **Repository Migration** ⭐ **NEW**
- **Bitbucket → GitHub Migration**: Complete repository migration from Bitbucket Server to GitHub
- **Mirror Clone**: Preserves complete git history including all branches and tags
- **Team Management**: Automatically assigns GitHub teams with proper permissions
- **Default Branch Setup**: Renames master to main and updates repository settings
- **Webhook Integration**: Configures CI/CD pipeline webhooks automatically
- **Real-Time Progress**: Step-by-step migration tracking with detailed status

### 🎯 **Platform & Interface**
- **Multi-Platform Support**: Works with GitHub.com, GitHub Enterprise, and Bitbucket Server
- **Dual Interface**: Professional CLI with interactive spinners + native desktop GUI (Fyne)
- **Real-Time Progress**: Interactive spinners (CLI) and progress bars (GUI)
- **Error Handling**: Comprehensive validation and recovery mechanisms

## Installation

### Prerequisites

- Go 1.24+
- Git
- Fyne dependencies (for native GUI - auto-installed with Go modules)

### Option 1: CLI + Fyne Native GUI (Recommended)
```bash
git clone https://github.com/your-org/go-toolgit.git
cd go-toolgit
go build -o go-toolgit

# Now you have both:
# CLI: ./go-toolgit --help
# Native GUI: ./go-toolgit --gui
```

### Option 2: CLI Only
```bash
git clone https://github.com/your-org/go-toolgit.git
cd go-toolgit
go build -o go-toolgit

# Use CLI interface only
./go-toolgit --help
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

### 5. Repository Migration (Optional)

```bash
# Migrate a repository from Bitbucket to GitHub
./go-toolgit migrate \
  --source-bitbucket-url "ssh://git@bitbucket.company.com:2222/PROJ/repo.git" \
  --target-github-org "my-org" \
  --target-repo-name "migrated-repo"
```

### 6. Launch GUI

Choose between two GUI options:

#### Fyne Native GUI
```bash
# Works with standard Go build - no additional setup needed
./go-toolgit --gui
```

#### CLI Interface
```bash
# Use command-line interface
./go-toolgit --help
./go-toolgit migrate --help
```

## CLI Usage

### Commands

#### String Replacement Commands
| Command | Description |
|---------|-------------|
| `replace` | Perform string replacements across GitHub repositories |
| `bitbucket-replace` | Perform string replacements across Bitbucket repositories |
| `replace-search` | Replace strings in repositories found by search |

#### Repository Migration Commands ⭐ **NEW**
| Command | Description |
|---------|-------------|
| `migrate` | Migrate repository from Bitbucket Server to GitHub |

#### Utility Commands
| Command | Description |
|---------|-------------|
| `validate` | Validate configuration and provider access (GitHub/Bitbucket) |
| `list` | List repositories accessible to the team/project |
| `search` | Search for repositories on GitHub using various criteria |
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

### Migration Command Options ⭐ **NEW**

| Flag | Description | Example |
|------|-------------|---------|
| `--source-bitbucket-url` | Source Bitbucket repository URL | `--source-bitbucket-url "ssh://git@bitbucket.company.com:2222/PROJ/repo.git"` |
| `--target-github-org` | Target GitHub organization | `--target-github-org "my-org"` |
| `--target-repo-name` | Target repository name (optional) | `--target-repo-name "new-repo-name"` |
| `--webhook-url` | Webhook URL for CI/CD integration | `--webhook-url "https://ci.company.com/webhook"` |
| `--teams` | Team assignments with permissions | `--teams "team1=pull,team2=maintain,team3=admin"` |
| `--dry-run` | Preview migration without execution | `--dry-run` |

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

Modern native GUI interface for both string replacement and repository migration:

### Fyne Native GUI
- **Native Performance**: True native desktop application
- **Easy Deployment**: Works with standard Go build, no additional setup
- **Cross-Platform**: Runs on Windows, macOS, and Linux
- **All Features**: Complete migration and replacement functionality

### String Replacement Features
- **Configuration Management**: Easy setup of GitHub/Bitbucket connection
- **Repository Selection**: Visual selection of target repositories
- **Rule Builder**: Interactive creation of replacement rules
- **Progress Tracking**: Real-time processing status
- **Results Display**: Detailed results with PR links

### Repository Migration Features ⭐ **NEW**
- **Migration Wizard**: Step-by-step repository migration setup
- **Source & Target Configuration**: Bitbucket source and GitHub target settings
- **Team Permission Management**: Interactive team assignment with permission levels
- **Webhook Configuration**: CI/CD pipeline integration setup
- **Real-Time Migration Progress**: Live tracking of all 9 migration steps
- **Migration Results**: Complete summary with GitHub repository URL and team assignments

Launch option:
- **Fyne Native GUI**: `./go-toolgit --gui` (works with standard build)

## Examples

### Repository Migration Examples ⭐ **NEW**

#### Basic Migration
```bash
# Migrate single repository from Bitbucket to GitHub
./go-toolgit migrate \
  --source-bitbucket-url "ssh://git@bitbucket.company.com:2222/PROJ/my-repo.git" \
  --target-github-org "my-github-org" \
  --target-repo-name "my-repo"
```

#### Advanced Migration with Teams and Webhook
```bash
# Complete migration setup with team permissions and CI/CD integration
./go-toolgit migrate \
  --source-bitbucket-url "ssh://git@bitbucket.company.com:2222/PROJ/backend-service.git" \
  --target-github-org "engineering" \
  --target-repo-name "backend-service" \
  --webhook-url "https://ci.company.com/github-webhook" \
  --teams "backend-team=maintain,devops-team=admin,qa-team=pull"
```

#### Migration with Dry Run
```bash
# Preview migration without making actual changes
./go-toolgit migrate \
  --source-bitbucket-url "ssh://git@bitbucket.company.com:2222/PROJ/legacy-app.git" \
  --target-github-org "platform" \
  --dry-run
```

### String Replacement Examples

#### Basic String Replacement
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
- Git

### Building

```bash
# CLI and GUI (string replacement, migration commands)
go build -o go-toolgit

# Note: The binary includes both CLI and GUI capabilities
```

### Running the Application

```bash
# CLI Commands
./go-toolgit validate
./go-toolgit migrate --source-bitbucket-url "..." --target-github-org "..."
./go-toolgit replace --replacements "old=new"

# GUI Interface
./go-toolgit --gui
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
├── internal/fyne-gui/    # Fyne native GUI implementation
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
- [Fyne](https://fyne.io/) - GUI framework
- [yacspin](https://github.com/theckman/yacspin) - CLI spinners
- [Viper](https://github.com/spf13/viper) - Configuration management