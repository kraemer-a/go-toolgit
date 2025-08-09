package processor

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go-toolgit/internal/core/git"
)

// MemoryProcessor processes repositories using in-memory git operations
type MemoryProcessor struct {
	engine    *ReplacementEngine
	gitOps    *git.MemoryOperations
	gitClient interface{} // GitHub client for PR creation
}

// MemoryProcessResult contains the results of processing a repository
type MemoryProcessResult struct {
	Repository   string
	Branch       string
	CommitHash   string
	FilesChanged []string
	FileChanges  []*FileChange // Detailed changes for dry-run
	Replacements int
	Success      bool
	Error        error
}

// NewMemoryProcessor creates a new in-memory git-based processor
func NewMemoryProcessor(engine *ReplacementEngine, gitOps *git.MemoryOperations) *MemoryProcessor {
	return &MemoryProcessor{
		engine: engine,
		gitOps: gitOps,
	}
}

// ProcessRepository processes a single repository using in-memory git operations
func (p *MemoryProcessor) ProcessRepository(ctx context.Context, repoURL, fullName, branchPrefix string, dryRun bool) (*MemoryProcessResult, error) {
	return p.ProcessRepositoryWithOptions(ctx, repoURL, fullName, branchPrefix, dryRun, false)
}

// ProcessRepositoryWithOptions processes a single repository with direct push option
func (p *MemoryProcessor) ProcessRepositoryWithOptions(ctx context.Context, repoURL, fullName, branchPrefix string, dryRun, directPush bool) (*MemoryProcessResult, error) {
	result := &MemoryProcessResult{
		Repository: fullName,
		Success:    false,
	}

	// Clone repository into memory
	memRepo, err := p.gitOps.CloneRepository(ctx, repoURL, fullName)
	if err != nil {
		result.Error = fmt.Errorf("failed to clone repository: %w", err)
		return result, result.Error
	}

	// List all files in the repository
	files, err := memRepo.ListFiles()
	if err != nil {
		result.Error = fmt.Errorf("failed to list files: %w", err)
		return result, result.Error
	}

	// Filter files based on include/exclude patterns
	filteredFiles := p.filterFiles(files)

	// Process files for replacements
	var modifiedFiles []git.FileInfo
	var changedFilenames []string
	var fileChanges []*FileChange
	totalReplacements := 0

	for _, file := range filteredFiles {
		// Apply replacements to file content
		originalContent := string(file.Content)
		modifiedContent, replacements, changes := p.processContentWithChanges(originalContent, file.Path, dryRun)

		if replacements > 0 {
			file.Content = []byte(modifiedContent)
			modifiedFiles = append(modifiedFiles, file)
			changedFilenames = append(changedFilenames, file.Path)
			totalReplacements += replacements

			if dryRun && changes != nil {
				fileChanges = append(fileChanges, changes)
			}
		}
	}

	result.FilesChanged = changedFilenames
	result.FileChanges = fileChanges
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

	// Update files in the repository
	err = memRepo.UpdateFiles(modifiedFiles)
	if err != nil {
		result.Error = fmt.Errorf("failed to update files: %w", err)
		return result, result.Error
	}

	// Check if there are actual changes in git
	hasChanges, err := memRepo.HasChanges()
	if err != nil {
		result.Error = fmt.Errorf("failed to check for changes: %w", err)
		return result, result.Error
	}

	if !hasChanges {
		result.Success = true
		return result, nil
	}

	// Prepare commit message
	commitMessage := fmt.Sprintf("Replace strings across %d files\n\nAutomated replacement by go-toolgit tool\n\n- Files modified: %d\n- Total replacements: %d",
		len(modifiedFiles), len(modifiedFiles), totalReplacements)

	if directPush {
		// Direct push mode: commit and push directly to default branch
		err = memRepo.CommitAndPushToDefault(ctx, commitMessage)
		if err != nil {
			result.Error = fmt.Errorf("failed to commit and push to default branch: %w", err)
			return result, result.Error
		}
		// No branch name for direct push
		result.Branch = ""
	} else {
		// Pull request mode: create new branch and push
		branchName := p.gitOps.GenerateBranchName(branchPrefix)

		err = memRepo.CreateBranchAndCommit(branchName, commitMessage)
		if err != nil {
			result.Error = fmt.Errorf("failed to create branch and commit: %w", err)
			return result, result.Error
		}

		result.Branch = branchName

		// Push the branch to remote
		err = memRepo.Push(ctx, branchName)
		if err != nil {
			result.Error = fmt.Errorf("failed to push branch: %w", err)
			return result, result.Error
		}
	}

	result.Success = true
	return result, nil
}

// filterFiles filters files based on include/exclude patterns
func (p *MemoryProcessor) filterFiles(files []git.FileInfo) []git.FileInfo {
	var filtered []git.FileInfo

	for _, file := range files {
		// Check exclude patterns first
		excluded := false
		for _, pattern := range p.engine.excludePatterns {
			if matched, _ := filepath.Match(pattern, file.Path); matched {
				excluded = true
				break
			}
			// Also check if any parent directory matches the pattern
			if strings.Contains(pattern, "/") && strings.HasPrefix(file.Path, strings.TrimSuffix(pattern, "*")) {
				excluded = true
				break
			}
		}

		if excluded {
			continue
		}

		// Check include patterns
		included := len(p.engine.includePatterns) == 0 // If no include patterns, include all (unless excluded)
		for _, pattern := range p.engine.includePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(file.Path)); matched {
				included = true
				break
			}
		}

		if included {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// processContent applies all replacement rules to content and returns modified content and replacement count
func (p *MemoryProcessor) processContent(content, filename string) (string, int) {
	modifiedContent, replacements, _ := p.processContentWithChanges(content, filename, false)
	return modifiedContent, replacements
}

// processContentWithChanges applies all replacement rules and optionally tracks detailed changes
func (p *MemoryProcessor) processContentWithChanges(content, filename string, trackChanges bool) (string, int, *FileChange) {
	modifiedContent := content
	totalReplacements := 0
	var stringChanges []StringChange

	for _, rule := range p.engine.rules {
		var replaced string
		var count int
		var changes []StringChange

		if trackChanges {
			changes = p.engine.findChanges(modifiedContent, rule)
			stringChanges = append(stringChanges, changes...)
		}

		if rule.Regex && rule.compiled != nil {
			replaced = rule.compiled.ReplaceAllString(modifiedContent, rule.Replacement)
			if !trackChanges {
				count = len(rule.compiled.FindAllString(modifiedContent, -1))
			} else {
				count = len(changes)
			}
		} else {
			if rule.CaseSensitive {
				if rule.WholeWord {
					replaced, count = p.replaceWholeWords(modifiedContent, rule.Original, rule.Replacement)
				} else {
					if !trackChanges {
						count = strings.Count(modifiedContent, rule.Original)
					} else {
						count = len(changes)
					}
					replaced = strings.ReplaceAll(modifiedContent, rule.Original, rule.Replacement)
				}
			} else {
				if rule.WholeWord {
					replaced, count = p.replaceWholeWordsInsensitive(modifiedContent, rule.Original, rule.Replacement)
				} else {
					if !trackChanges {
						lower := strings.ToLower(modifiedContent)
						lowerOriginal := strings.ToLower(rule.Original)
						count = strings.Count(lower, lowerOriginal)
					} else {
						count = len(changes)
					}
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

	var fileChange *FileChange
	if trackChanges && totalReplacements > 0 {
		fileChange = &FileChange{
			FilePath:      filename,
			OriginalSize:  int64(len(content)),
			ModifiedSize:  int64(len(modifiedContent)),
			Replacements:  totalReplacements,
			StringChanges: stringChanges,
		}
	}

	return modifiedContent, totalReplacements, fileChange
}

// Helper methods for string replacement (copied from ReplacementEngine)
func (p *MemoryProcessor) replaceWholeWords(content, original, replacement string) (string, int) {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(original) + `\b`)
	matches := pattern.FindAllString(content, -1)
	return pattern.ReplaceAllString(content, replacement), len(matches)
}

func (p *MemoryProcessor) replaceWholeWordsInsensitive(content, original, replacement string) (string, int) {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(original) + `\b`)
	matches := pattern.FindAllString(content, -1)
	return pattern.ReplaceAllString(content, replacement), len(matches)
}

func (p *MemoryProcessor) replaceAllInsensitive(content, original, replacement string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(original))
	return re.ReplaceAllString(content, replacement)
}
