package processor

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go-toolgit/internal/core/git"
)

// FileProcessor processes file operations (move/rename) across repositories
type FileProcessor struct {
	gitOps *git.MemoryOperations
}

// FileOperationRule defines a file operation
type FileOperationRule struct {
	SourcePath    string
	TargetPath    string
	SearchMode    string // "exact" or "filename"
	OperationType string // "move" or "rename"
}

// FileProcessResult contains the results of processing file operations
type FileProcessResult struct {
	Repository   string
	Branch       string
	CommitHash   string
	FilesChanged []string
	FileMatches  []string // Files that matched the pattern
	Success      bool
	Error        error
}

// NewFileProcessor creates a new file processor
func NewFileProcessor(gitOps *git.MemoryOperations) *FileProcessor {
	return &FileProcessor{
		gitOps: gitOps,
	}
}

// ProcessRepository processes file operations for a single repository
func (p *FileProcessor) ProcessRepository(ctx context.Context, repoURL, fullName, branchPrefix string, rules []FileOperationRule, dryRun, directPush bool) (*FileProcessResult, error) {
	result := &FileProcessResult{
		Repository: fullName,
		Success:    false,
	}

	// Clone repository into memory
	memRepo, err := p.gitOps.CloneRepository(ctx, repoURL, fullName)
	if err != nil {
		result.Error = fmt.Errorf("failed to clone repository: %w", err)
		return result, result.Error
	}

	var allChangedFiles []string
	var allMatchedFiles []string

	// Process each rule
	for _, rule := range rules {
		// Find matching files
		matches, err := memRepo.FindFiles(rule.SourcePath, rule.SearchMode)
		if err != nil {
			result.Error = fmt.Errorf("failed to find files: %w", err)
			return result, result.Error
		}

		allMatchedFiles = append(allMatchedFiles, matches...)

		// Skip if no matches found
		if len(matches) == 0 {
			continue
		}

		// Process each matched file
		for _, matchedFile := range matches {
			targetPath := p.calculateTargetPath(matchedFile, rule)

			// Skip if source and target are the same
			if matchedFile == targetPath {
				continue
			}

			if !dryRun {
				// Perform the actual file operation
				if rule.OperationType == "move" || rule.OperationType == "rename" {
					err = memRepo.MoveFile(matchedFile, targetPath)
					if err != nil {
						result.Error = fmt.Errorf("failed to move file %s to %s: %w", matchedFile, targetPath, err)
						return result, result.Error
					}
				}
			}

			allChangedFiles = append(allChangedFiles, fmt.Sprintf("%s → %s", matchedFile, targetPath))
		}
	}

	result.FilesChanged = allChangedFiles
	result.FileMatches = allMatchedFiles

	// If no changes, return success
	if len(allChangedFiles) == 0 {
		result.Success = true
		return result, nil
	}

	// If dry run, don't make actual changes
	if dryRun {
		result.Success = true
		return result, nil
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
	commitMessage := fmt.Sprintf("Move/rename %d file(s)\n\nAutomated file operations by go-toolgit tool\n\nFiles affected:\n%s",
		len(allChangedFiles), strings.Join(allChangedFiles, "\n"))

	if directPush {
		// Direct push mode: commit and push directly to default branch
		commitHash, err := memRepo.CommitAndPushToDefault(ctx, commitMessage)
		if err != nil {
			result.Error = fmt.Errorf("failed to commit and push to default branch: %w", err)
			return result, result.Error
		}
		result.CommitHash = commitHash
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

// calculateTargetPath calculates the target path for a file operation
func (p *FileProcessor) calculateTargetPath(sourcePath string, rule FileOperationRule) string {
	// If target contains wildcards or is a pattern, replace filename only
	if strings.Contains(rule.TargetPath, "*") {
		// For patterns, replace the filename part
		sourceBase := filepath.Base(sourcePath)
		targetBase := filepath.Base(rule.TargetPath)

		// Simple pattern replacement (e.g., *.yml -> *.yaml)
		if strings.HasPrefix(targetBase, "*") && strings.HasPrefix(filepath.Base(rule.SourcePath), "*") {
			sourceExt := filepath.Ext(sourceBase)
			targetExt := filepath.Ext(targetBase)
			if sourceExt != "" && targetExt != "" {
				nameWithoutExt := strings.TrimSuffix(sourceBase, sourceExt)
				newName := nameWithoutExt + targetExt
				return filepath.Join(filepath.Dir(sourcePath), newName)
			}
		}
	}

	// If search mode is "filename", preserve directory structure
	if rule.SearchMode == "filename" {
		// Replace just the filename, keep the directory
		return filepath.Join(filepath.Dir(sourcePath), filepath.Base(rule.TargetPath))
	}

	// For exact mode, use target path as-is
	return rule.TargetPath
}

// ProcessRepositoryWithAuth processes file operations for a single repository using basic auth
func (p *FileProcessor) ProcessRepositoryWithAuth(ctx context.Context, repoURL, fullName, username, password, branchPrefix string, rules []FileOperationRule, dryRun, directPush bool) (*FileProcessResult, error) {
	result := &FileProcessResult{
		Repository: fullName,
		Success:    false,
	}

	// Clone repository into memory using basic auth
	memRepo, err := p.gitOps.CloneRepositoryWithBasicAuth(ctx, repoURL, fullName, username, password)
	if err != nil {
		result.Error = fmt.Errorf("failed to clone repository: %w", err)
		return result, result.Error
	}

	var allChangedFiles []string
	var allMatchedFiles []string

	// Process each rule
	for _, rule := range rules {
		// Find matching files
		matches, err := memRepo.FindFiles(rule.SourcePath, rule.SearchMode)
		if err != nil {
			result.Error = fmt.Errorf("failed to find files: %w", err)
			return result, result.Error
		}

		allMatchedFiles = append(allMatchedFiles, matches...)

		// Skip if no matches found
		if len(matches) == 0 {
			continue
		}

		// Process each matched file
		for _, matchedFile := range matches {
			targetPath := p.calculateTargetPath(matchedFile, rule)

			// Skip if source and target are the same
			if matchedFile == targetPath {
				continue
			}

			if !dryRun {
				// Perform the actual file operation
				if rule.OperationType == "move" || rule.OperationType == "rename" {
					err = memRepo.MoveFile(matchedFile, targetPath)
					if err != nil {
						result.Error = fmt.Errorf("failed to move file %s to %s: %w", matchedFile, targetPath, err)
						return result, result.Error
					}
				}
			}

			allChangedFiles = append(allChangedFiles, fmt.Sprintf("%s → %s", matchedFile, targetPath))
		}
	}

	result.FilesChanged = allChangedFiles
	result.FileMatches = allMatchedFiles

	// If no changes, return success
	if len(allChangedFiles) == 0 {
		result.Success = true
		return result, nil
	}

	// If dry run, don't make actual changes
	if dryRun {
		result.Success = true
		return result, nil
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
	commitMessage := fmt.Sprintf("Move/rename %d file(s)\n\nAutomated file operations by go-toolgit tool\n\nFiles affected:\n%s",
		len(allChangedFiles), strings.Join(allChangedFiles, "\n"))

	if directPush {
		// Direct push mode: commit and push directly to default branch
		commitHash, err := memRepo.CommitAndPushToDefault(ctx, commitMessage)
		if err != nil {
			result.Error = fmt.Errorf("failed to commit and push to default branch: %w", err)
			return result, result.Error
		}
		result.CommitHash = commitHash
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

// FindFilesInRepository finds files matching patterns in a repository (for preview)
func (p *FileProcessor) FindFilesInRepository(ctx context.Context, repoURL, fullName string, rules []FileOperationRule) ([]string, error) {
	// Clone repository into memory
	memRepo, err := p.gitOps.CloneRepository(ctx, repoURL, fullName)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	var allMatches []string
	for _, rule := range rules {
		matches, err := memRepo.FindFiles(rule.SourcePath, rule.SearchMode)
		if err != nil {
			return nil, fmt.Errorf("failed to find files: %w", err)
		}
		allMatches = append(allMatches, matches...)
	}

	return allMatches, nil
}
