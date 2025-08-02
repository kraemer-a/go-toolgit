package github

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
)

// RepositoryOperations handles repository operations using GitHub API instead of git binary
type RepositoryOperations struct {
	client *Client
}

// FileContent represents a file and its content
type FileContent struct {
	Path    string
	Content []byte
	SHA     string // GitHub requires SHA for updates
}

// BranchInfo contains information about a created branch
type BranchInfo struct {
	Name string
	SHA  string
}

// NewRepositoryOperations creates a new repository operations handler
func NewRepositoryOperations(client *Client) *RepositoryOperations {
	return &RepositoryOperations{
		client: client,
	}
}

// DownloadRepository downloads all files from a repository
func (r *RepositoryOperations) DownloadRepository(ctx context.Context, owner, repo, ref string) ([]FileContent, error) {
	// Get the repository tree recursively
	tree, _, err := r.client.client.Git.GetTree(ctx, owner, repo, ref, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	var files []FileContent

	// Download each file
	for _, entry := range tree.Entries {
		// Skip directories and non-blob entries
		if entry.GetType() != "blob" {
			continue
		}

		// Get file content
		fileContent, _, _, err := r.client.client.Repositories.GetContents(ctx, owner, repo, entry.GetPath(), &github.RepositoryContentGetOptions{
			Ref: ref,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get file content for %s: %w", entry.GetPath(), err)
		}

		content, err := fileContent.GetContent()
		if err != nil {
			return nil, fmt.Errorf("failed to decode file content for %s: %w", entry.GetPath(), err)
		}

		files = append(files, FileContent{
			Path:    entry.GetPath(),
			Content: []byte(content),
			SHA:     fileContent.GetSHA(),
		})
	}

	return files, nil
}

// CreateBranch creates a new branch from the default branch
func (r *RepositoryOperations) CreateBranch(ctx context.Context, owner, repo, branchName string) (*BranchInfo, error) {
	// Get the default branch SHA
	repoInfo, _, err := r.client.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := repoInfo.GetDefaultBranch()

	// Get the SHA of the default branch
	ref, _, err := r.client.client.Git.GetRef(ctx, owner, repo, "refs/heads/"+defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get default branch ref: %w", err)
	}

	baseSHA := ref.Object.GetSHA()

	// Create new branch
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + branchName),
		Object: &github.GitObject{
			SHA: github.String(baseSHA),
		},
	}

	_, _, err = r.client.client.Git.CreateRef(ctx, owner, repo, newRef)
	if err != nil {
		return nil, fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}

	return &BranchInfo{
		Name: branchName,
		SHA:  baseSHA,
	}, nil
}

// UpdateFiles updates multiple files in the repository
func (r *RepositoryOperations) UpdateFiles(ctx context.Context, owner, repo, branch string, files []FileContent, commitMessage string) error {
	for _, file := range files {
		// Check if file exists to determine if we should create or update
		existingFile, _, _, err := r.client.client.Repositories.GetContents(ctx, owner, repo, file.Path, &github.RepositoryContentGetOptions{
			Ref: branch,
		})

		opts := &github.RepositoryContentFileOptions{
			Message: github.String(commitMessage),
			Content: file.Content,
			Branch:  github.String(branch),
			Committer: &github.CommitAuthor{
				Name:  github.String("GitHub Replace Tool"),
				Email: github.String("go-toolgit@automated.tool"),
			},
		}

		if err == nil && existingFile != nil {
			// File exists - update it
			opts.SHA = github.String(existingFile.GetSHA())
			_, _, err = r.client.client.Repositories.UpdateFile(ctx, owner, repo, file.Path, opts)
		} else {
			// File doesn't exist - create it
			_, _, err = r.client.client.Repositories.CreateFile(ctx, owner, repo, file.Path, opts)
		}

		if err != nil {
			return fmt.Errorf("failed to update file %s: %w", file.Path, err)
		}
	}

	return nil
}

// GenerateBranchName generates a unique branch name with timestamp
func (r *RepositoryOperations) GenerateBranchName(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", prefix, timestamp)
}

// FilterFiles filters files based on include/exclude patterns
func FilterFiles(files []FileContent, includePatterns, excludePatterns []string) []FileContent {
	var filtered []FileContent

	for _, file := range files {
		// Check exclude patterns first
		excluded := false
		for _, pattern := range excludePatterns {
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
		included := len(includePatterns) == 0 // If no include patterns, include all (unless excluded)
		for _, pattern := range includePatterns {
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
