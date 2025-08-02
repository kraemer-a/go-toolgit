package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Operations struct {
	workingDir string
	gitPath    string
	token      string // GitHub PAT for authentication
}

type Repository struct {
	URL       string
	LocalPath string
	Branch    string
}

type CommitOptions struct {
	Message string
	Author  string
	Email   string
}

func NewOperations() (*Operations, error) {
	return NewOperationsWithToken("")
}

func NewOperationsWithToken(token string) (*Operations, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found in PATH: %w", err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	return &Operations{
		workingDir: workingDir,
		gitPath:    gitPath,
		token:      token,
	}, nil
}

func (g *Operations) CloneRepository(repoURL, localPath string) error {
	if err := os.RemoveAll(localPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	// If we have a token and this is a GitHub HTTPS URL, add authentication
	authenticatedURL := g.addTokenToURL(repoURL)

	cmd := exec.Command(g.gitPath, "clone", authenticatedURL, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w, output: %s", err, string(output))
	}

	return nil
}

// addTokenToURL adds the GitHub PAT to the URL for authentication
func (g *Operations) addTokenToURL(repoURL string) string {
	if g.token == "" {
		return repoURL
	}

	// Handle GitHub HTTPS URLs
	if strings.HasPrefix(repoURL, "https://github.com/") {
		// Convert https://github.com/user/repo.git to https://token@github.com/user/repo.git
		return strings.Replace(repoURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", g.token), 1)
	}

	// Handle GitHub Enterprise URLs (if base URL is different)
	if strings.Contains(repoURL, "github") && strings.HasPrefix(repoURL, "https://") {
		// For enterprise GitHub: https://github.company.com/user/repo.git -> https://token@github.company.com/user/repo.git
		parts := strings.SplitN(repoURL, "://", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("%s://%s@%s", parts[0], g.token, parts[1])
		}
	}

	return repoURL
}

func (g *Operations) CreateBranch(repoPath, branchName string) error {
	cmd := exec.Command(g.gitPath, "checkout", "-b", branchName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w, output: %s", branchName, err, string(output))
	}

	return nil
}

func (g *Operations) AddAllChanges(repoPath string) error {
	cmd := exec.Command(g.gitPath, "add", ".")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add changes: %w, output: %s", err, string(output))
	}

	return nil
}

func (g *Operations) Commit(repoPath string, options CommitOptions) error {
	args := []string{"commit", "-m", options.Message}

	if options.Author != "" && options.Email != "" {
		args = append(args, "--author", fmt.Sprintf("%s <%s>", options.Author, options.Email))
	}

	cmd := exec.Command(g.gitPath, args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w, output: %s", err, string(output))
	}

	return nil
}

func (g *Operations) Push(repoPath, branchName string) error {
	// Ensure the remote URL has authentication if we have a token
	if g.token != "" {
		if err := g.updateRemoteURL(repoPath); err != nil {
			return fmt.Errorf("failed to update remote URL: %w", err)
		}
	}

	cmd := exec.Command(g.gitPath, "push", "-u", "origin", branchName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w, output: %s", branchName, err, string(output))
	}

	return nil
}

// updateRemoteURL ensures the remote origin URL has authentication
func (g *Operations) updateRemoteURL(repoPath string) error {
	// Get current remote URL
	cmd := exec.Command(g.gitPath, "config", "--get", "remote.origin.url")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get remote URL: %w", err)
	}

	currentURL := strings.TrimSpace(string(output))
	authenticatedURL := g.addTokenToURL(currentURL)

	// Only update if the URL changed (i.e., token was added)
	if authenticatedURL != currentURL {
		cmd = exec.Command(g.gitPath, "remote", "set-url", "origin", authenticatedURL)
		cmd.Dir = repoPath
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to update remote URL: %w, output: %s", err, string(output))
		}
	}

	return nil
}

func (g *Operations) HasChanges(repoPath string) (bool, error) {
	cmd := exec.Command(g.gitPath, "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func (g *Operations) GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command(g.gitPath, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (g *Operations) GenerateBranchName(prefix string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", prefix, timestamp)
}

func (g *Operations) ConfigureUser(repoPath, name, email string) error {
	nameCmd := exec.Command(g.gitPath, "config", "user.name", name)
	nameCmd.Dir = repoPath
	if output, err := nameCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure git user name: %w, output: %s", err, string(output))
	}

	emailCmd := exec.Command(g.gitPath, "config", "user.email", email)
	emailCmd.Dir = repoPath
	if output, err := emailCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure git user email: %w, output: %s", err, string(output))
	}

	return nil
}

func (g *Operations) CleanupRepository(repoPath string) error {
	if repoPath == "" || repoPath == "/" || repoPath == g.workingDir {
		return fmt.Errorf("refusing to delete working directory or root")
	}

	return os.RemoveAll(repoPath)
}

func (g *Operations) GetRepositoryInfo(repoPath string) (*Repository, error) {
	urlCmd := exec.Command(g.gitPath, "config", "--get", "remote.origin.url")
	urlCmd.Dir = repoPath
	urlOutput, err := urlCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository URL: %w", err)
	}

	branch, err := g.GetCurrentBranch(repoPath)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &Repository{
		URL:       strings.TrimSpace(string(urlOutput)),
		LocalPath: absPath,
		Branch:    branch,
	}, nil
}
