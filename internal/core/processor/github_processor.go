package processor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go-toolgit/internal/core/github"
)

// GitHubProcessor processes repositories using GitHub API instead of local git operations
type GitHubProcessor struct {
	engine  *ReplacementEngine
	repoOps *github.RepositoryOperations
}

// ProcessResult contains the results of processing a repository
type ProcessResult struct {
	Repository   string
	Branch       string
	FilesChanged []string
	Replacements int
	Success      bool
	Error        error
}

// NewGitHubProcessor creates a new GitHub API-based processor
func NewGitHubProcessor(engine *ReplacementEngine, repoOps *github.RepositoryOperations) *GitHubProcessor {
	return &GitHubProcessor{
		engine:  engine,
		repoOps: repoOps,
	}
}

// ProcessRepository processes a single repository using GitHub API
func (p *GitHubProcessor) ProcessRepository(ctx context.Context, owner, repo string, branchPrefix string, dryRun bool) (*ProcessResult, error) {
	result := &ProcessResult{
		Repository: fmt.Sprintf("%s/%s", owner, repo),
		Success:    false,
	}

	// Download repository files
	files, err := p.repoOps.DownloadRepository(ctx, owner, repo, "HEAD")
	if err != nil {
		result.Error = fmt.Errorf("failed to download repository: %w", err)
		return result, result.Error
	}

	// Filter files based on include/exclude patterns
	filteredFiles := github.FilterFiles(files, p.engine.includePatterns, p.engine.excludePatterns)

	// Process files for replacements
	var modifiedFiles []github.FileContent
	var changedFilenames []string
	totalReplacements := 0

	for _, file := range filteredFiles {
		// Apply replacements to file content
		originalContent := string(file.Content)
		modifiedContent, replacements := p.processContent(originalContent, file.Path)

		if replacements > 0 {
			file.Content = []byte(modifiedContent)
			modifiedFiles = append(modifiedFiles, file)
			changedFilenames = append(changedFilenames, file.Path)
			totalReplacements += replacements
		}
	}

	result.FilesChanged = changedFilenames
	result.Replacements = totalReplacements

	// If no changes, return success
	if len(modifiedFiles) == 0 {
		result.Success = true
		return result, nil
	}

	// If dry run, don't make actual changes
	if dryRun {
		result.Success = true
		return result, nil
	}

	// Create a new branch
	branchName := p.repoOps.GenerateBranchName(branchPrefix)
	branchInfo, err := p.repoOps.CreateBranch(ctx, owner, repo, branchName)
	if err != nil {
		result.Error = fmt.Errorf("failed to create branch: %w", err)
		return result, result.Error
	}

	result.Branch = branchInfo.Name

	// Update files in the new branch
	commitMessage := fmt.Sprintf("Replace strings across %d files\n\nAutomated replacement by go-toolgit tool", len(modifiedFiles))
	err = p.repoOps.UpdateFiles(ctx, owner, repo, branchName, modifiedFiles, commitMessage)
	if err != nil {
		result.Error = fmt.Errorf("failed to update files: %w", err)
		return result, result.Error
	}

	result.Success = true
	return result, nil
}

// processContent applies all replacement rules to content and returns modified content and replacement count
func (p *GitHubProcessor) processContent(content, filename string) (string, int) {
	modifiedContent := content
	totalReplacements := 0

	for _, rule := range p.engine.rules {
		var replaced string
		var count int

		if rule.Regex && rule.compiled != nil {
			replaced = rule.compiled.ReplaceAllString(modifiedContent, rule.Replacement)
			count = len(rule.compiled.FindAllString(modifiedContent, -1))
		} else {
			if rule.CaseSensitive {
				if rule.WholeWord {
					replaced, count = p.replaceWholeWords(modifiedContent, rule.Original, rule.Replacement)
				} else {
					count = strings.Count(modifiedContent, rule.Original)
					replaced = strings.ReplaceAll(modifiedContent, rule.Original, rule.Replacement)
				}
			} else {
				lower := strings.ToLower(modifiedContent)
				lowerOriginal := strings.ToLower(rule.Original)
				if rule.WholeWord {
					replaced, count = p.replaceWholeWordsInsensitive(modifiedContent, rule.Original, rule.Replacement)
				} else {
					count = strings.Count(lower, lowerOriginal)
					replaced = p.replaceAllInsensitive(modifiedContent, rule.Original, rule.Replacement)
				}
			}
		}

		modifiedContent = replaced
		totalReplacements += count
	}

	if totalReplacements > 0 {
		p.engine.stats.FilesModified++
	}
	p.engine.stats.FilesProcessed++

	return modifiedContent, totalReplacements
}

// Helper methods for string replacement (copied from ReplacementEngine)
func (p *GitHubProcessor) replaceWholeWords(content, original, replacement string) (string, int) {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(original) + `\b`)
	matches := pattern.FindAllString(content, -1)
	return pattern.ReplaceAllString(content, replacement), len(matches)
}

func (p *GitHubProcessor) replaceWholeWordsInsensitive(content, original, replacement string) (string, int) {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(original) + `\b`)
	matches := pattern.FindAllString(content, -1)
	return pattern.ReplaceAllString(content, replacement), len(matches)
}

func (p *GitHubProcessor) replaceAllInsensitive(content, original, replacement string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(original))
	return re.ReplaceAllString(content, replacement)
}
