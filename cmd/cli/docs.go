package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Generate markdown documentation",
	Long: `Generate markdown documentation for all commands.

This will create comprehensive markdown documentation for the CLI
including all commands, flags, and examples.`,
	Example: `  # Generate docs to default directory (./docs)
  go-toolgit docs

  # Generate docs to custom directory
  go-toolgit docs --output ./documentation

  # Generate docs with custom file prefix
  go-toolgit docs --output ./docs --filename-prefix go-toolgit`,
	RunE: generateDocs,
}

func init() {
	rootCmd.AddCommand(docsCmd)

	docsCmd.Flags().StringP("output", "o", "./docs", "Output directory for documentation")
	docsCmd.Flags().String("filename-prefix", "", "Prefix for generated filenames (default: command name)")
	docsCmd.Flags().Bool("include-date", false, "Include generation date in documentation")
}

func generateDocs(cmd *cobra.Command, args []string) error {
	outputDir, _ := cmd.Flags().GetString("output")
	filenamePrefix, _ := cmd.Flags().GetString("filename-prefix")
	includeDate, _ := cmd.Flags().GetBool("include-date")

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	fmt.Printf("Generating markdown documentation in %s...\n", outputDir)

	// Configure doc generation options
	if filenamePrefix != "" {
		// Generate docs with custom filename prefix
		err := doc.GenMarkdownTreeCustom(rootCmd, outputDir, func(filename string) string {
			return filenamePrefix + "_" + filename
		}, func(filename string) string {
			// Custom link handler
			return filename
		})
		if err != nil {
			return fmt.Errorf("failed to generate documentation: %w", err)
		}
	} else {
		// Generate docs with default naming
		err := doc.GenMarkdownTree(rootCmd, outputDir)
		if err != nil {
			return fmt.Errorf("failed to generate documentation: %w", err)
		}
	}

	// Generate a comprehensive index file
	if err := generateIndexFile(outputDir, includeDate); err != nil {
		return fmt.Errorf("failed to generate index file: %w", err)
	}

	fmt.Printf("Documentation successfully generated in %s\n", outputDir)
	fmt.Println("\nGenerated files:")

	// List generated files
	files, err := filepath.Glob(filepath.Join(outputDir, "*.md"))
	if err != nil {
		return fmt.Errorf("failed to list generated files: %w", err)
	}

	for _, file := range files {
		fmt.Printf("  - %s\n", filepath.Base(file))
	}

	return nil
}

func generateIndexFile(outputDir string, includeDate bool) error {
	indexPath := filepath.Join(outputDir, "README.md")

	content := `# GitHub & Bitbucket Replace Tool - CLI Documentation

This directory contains comprehensive documentation for the GitHub & Bitbucket Replace Tool CLI commands.

## Overview

The GitHub & Bitbucket Replace Tool is a powerful CLI and GUI application for performing automated string replacements across multiple repositories on GitHub and Bitbucket Server.

## Quick Start

` + "```bash" + `
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
` + "```" + `

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

1. **Configuration file**: ` + "`~/.go-toolgit/config.yaml`" + `
2. **Environment variables**: ` + "`GITHUB_REPLACE_*`" + ` prefix
3. **Command-line flags**: See individual command documentation

## Examples

### GitHub Examples

` + "```bash" + `
# Basic replacement
go-toolgit replace --org mycompany --team backend --replacements "oldAPI=newAPI"

# Multiple replacements with file patterns
go-toolgit replace --replacements "jQuery=React,$.ajax=fetch" --include "*.js,*.jsx"

# Enterprise GitHub
go-toolgit replace --github-url "https://github.company.com" --token "$TOKEN" --replacements "v1=v2"
` + "```" + `

### Bitbucket Examples

` + "```bash" + `
# Basic Bitbucket replacement
go-toolgit bitbucket-replace --bitbucket-url "https://bitbucket.company.com" \
  --bitbucket-username "user" --bitbucket-password "$TOKEN" \
  --bitbucket-project "PROJ" --replacements "oldFunction=newFunction"
` + "```" + `

### Search Examples

` + "```bash" + `
# Search repositories by owner
go-toolgit search --owner myorg

# Search and replace in one command
go-toolgit replace-search --owner myorg --language go --replacements "oldAPI=newAPI"
` + "```" + `

## Support

For issues and feature requests, please visit: https://github.com/your-org/go-toolgit

`

	if includeDate {
		content += fmt.Sprintf("\n---\n*Documentation generated on %s*\n",
			time.Now().Format("January 2, 2006"))
	}

	return os.WriteFile(indexPath, []byte(content), 0644)
}
