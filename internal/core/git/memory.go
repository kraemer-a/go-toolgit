package git

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

// MemoryRepository represents a repository cloned in memory
type MemoryRepository struct {
	repo     *git.Repository
	fs       billy.Filesystem
	auth     *http.BasicAuth
	repoURL  string
	fullName string
}

// MemoryOperations handles Git operations entirely in memory using go-git
type MemoryOperations struct {
	token string
}

// FileInfo represents a file in the repository
type FileInfo struct {
	Path    string
	Content []byte
	Mode    os.FileMode
}

// ProcessResult contains the results of processing a repository
type ProcessResult struct {
	Repository   string
	Branch       string
	CommitHash   string
	FilesChanged []string
	Replacements int
	Success      bool
	Error        error
}

// NewMemoryOperations creates a new in-memory git operations handler
func NewMemoryOperations(token string) *MemoryOperations {
	return &MemoryOperations{
		token: token,
	}
}

// CloneRepository clones a repository into memory
func (m *MemoryOperations) CloneRepository(ctx context.Context, repoURL, fullName string) (*MemoryRepository, error) {
	startTime := time.Now()
	
	storage := memory.NewStorage()
	fs := memfs.New()

	auth := &http.BasicAuth{
		Username: "git", // Can be anything for GitHub PAT
		Password: m.token,
	}

	repo, err := git.Clone(storage, fs, &git.CloneOptions{
		URL:  repoURL,
		Auth: auth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s: %w", fullName, err)
	}

	cloneDuration := time.Since(startTime)
	log.Printf("[INFO] Successfully cloned repository %s in %v", fullName, cloneDuration)

	return &MemoryRepository{
		repo:     repo,
		fs:       fs,
		auth:     auth,
		repoURL:  repoURL,
		fullName: fullName,
	}, nil
}

// ListFiles returns all files in the repository
func (mr *MemoryRepository) ListFiles() ([]FileInfo, error) {
	var files []FileInfo

	err := walkFiles(mr.fs, "/", func(path string, info os.FileInfo) error {
		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Read file content
		file, err := mr.fs.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		files = append(files, FileInfo{
			Path:    strings.TrimPrefix(path, "/"),
			Content: content,
			Mode:    info.Mode(),
		})

		return nil
	})

	return files, err
}

// UpdateFiles updates multiple files in the repository
func (mr *MemoryRepository) UpdateFiles(files []FileInfo) error {
	for _, file := range files {
		// Create directories if they don't exist
		dir := filepath.Dir(file.Path)
		if dir != "." {
			err := mr.fs.MkdirAll(dir, 0755)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Write file content
		f, err := mr.fs.Create(file.Path)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", file.Path, err)
		}

		_, err = f.Write(file.Content)
		f.Close()
		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
	}

	return nil
}

// CreateBranchAndCommit creates a new branch and commits the changes
func (mr *MemoryRepository) CreateBranchAndCommit(branchName, commitMessage string) error {
	// Get the working tree
	worktree, err := mr.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes to staging area
	_, err = worktree.Add(".")
	if err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Commit the changes to current branch first
	commit, err := worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "GitHub Replace Tool",
			Email: "go-toolgit@automated.tool",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Create new branch reference pointing to the new commit
	ref := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branchName), commit)
	err = mr.repo.Storer.SetReference(ref)
	if err != nil {
		return fmt.Errorf("failed to create branch reference: %w", err)
	}

	// Checkout the new branch
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branchName),
	})
	if err != nil {
		return fmt.Errorf("failed to checkout branch: %w", err)
	}

	// Only show commit info in debug mode - this would need to be passed as a parameter
	// For now, remove the output to avoid UI interference
	return nil
}

// Push pushes the branch to the remote repository
func (mr *MemoryRepository) Push(ctx context.Context, branchName string) error {
	// Push the branch
	err := mr.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName),
		},
		Auth: mr.auth,
	})
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branchName, err)
	}

	return nil
}

// HasChanges checks if there are any modified files
func (mr *MemoryRepository) HasChanges() (bool, error) {
	worktree, err := mr.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	return !status.IsClean(), nil
}

// GenerateBranchName generates a unique branch name with timestamp
func (m *MemoryOperations) GenerateBranchName(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", prefix, timestamp)
}

// walkFiles is a helper function to walk through files in the filesystem
func walkFiles(fs billy.Filesystem, root string, fn func(path string, info os.FileInfo) error) error {
	files, err := fs.ReadDir(root)
	if err != nil {
		return err
	}

	for _, file := range files {
		path := filepath.Join(root, file.Name())

		if file.IsDir() {
			// Skip .git directory and other hidden directories
			if strings.HasPrefix(file.Name(), ".") {
				continue
			}
			err = walkFiles(fs, path, fn)
			if err != nil {
				return err
			}
		} else {
			err = fn(path, file)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
