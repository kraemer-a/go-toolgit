package git

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"go-toolgit/internal/core/security"
)

// SecureOperations wraps git operations with input validation and security features
type SecureOperations struct {
	operations *Operations
	validator  *security.InputValidator
}

// NewSecureOperations creates a new secure git operations wrapper
func NewSecureOperations() (*SecureOperations, error) {
	ops, err := NewOperations()
	if err != nil {
		return nil, err
	}
	
	return &SecureOperations{
		operations: ops,
		validator:  security.NewInputValidator(true), // Use strict mode for git operations
	}, nil
}

// NewSecureOperationsWithToken creates secure git operations with a token
func NewSecureOperationsWithToken(token string) (*SecureOperations, error) {
	// Validate token first
	validator := security.NewInputValidator(true)
	if err := validator.ValidateToken("token", token); err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	
	// Sanitize token
	sanitizedToken := validator.SanitizeString(token)
	
	ops, err := NewOperationsWithToken(sanitizedToken)
	if err != nil {
		return nil, err
	}
	
	return &SecureOperations{
		operations: ops,
		validator:  validator,
	}, nil
}

// CloneRepository securely clones a repository with input validation
func (so *SecureOperations) CloneRepository(repoURL, localPath string) error {
	// Validate inputs
	if err := so.validator.ValidateURL("repo_url", repoURL); err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}
	
	if err := so.validator.ValidateFilePath("local_path", localPath); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}
	
	// Additional validation for repository URL
	if err := so.validateRepositoryURL(repoURL); err != nil {
		return fmt.Errorf("repository URL validation failed: %w", err)
	}
	
	// Validate local path is safe
	if err := so.validateClonePath(localPath); err != nil {
		return fmt.Errorf("clone path validation failed: %w", err)
	}
	
	// Sanitize inputs
	repoURL = so.validator.SanitizeString(repoURL)
	localPath = so.validator.SanitizeString(localPath)
	
	return so.operations.CloneRepository(repoURL, localPath)
}

// CreateBranch securely creates a git branch with validation
func (so *SecureOperations) CreateBranch(repoPath, branchName string) error {
	// Validate inputs
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	
	if err := so.validator.ValidateBranchName("branch_name", branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}
	
	// Additional validation for branch name
	if err := so.validateGitBranchName(branchName); err != nil {
		return fmt.Errorf("branch name validation failed: %w", err)
	}
	
	// Sanitize inputs
	repoPath = so.validator.SanitizeString(repoPath)
	branchName = so.validator.SanitizeString(branchName)
	
	return so.operations.CreateBranch(repoPath, branchName)
}

// AddAllChanges securely adds all changes with path validation
func (so *SecureOperations) AddAllChanges(repoPath string) error {
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	
	// Ensure path is within expected boundaries
	if err := so.validateWorkingDirectory(repoPath); err != nil {
		return fmt.Errorf("working directory validation failed: %w", err)
	}
	
	repoPath = so.validator.SanitizeString(repoPath)
	return so.operations.AddAllChanges(repoPath)
}

// Commit securely commits changes with validation
func (so *SecureOperations) Commit(repoPath string, options CommitOptions) error {
	// Validate inputs
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	
	if err := so.validateCommitOptions(options); err != nil {
		return fmt.Errorf("invalid commit options: %w", err)
	}
	
	// Sanitize inputs
	repoPath = so.validator.SanitizeString(repoPath)
	options = so.sanitizeCommitOptions(options)
	
	return so.operations.Commit(repoPath, options)
}

// Push securely pushes changes with validation
func (so *SecureOperations) Push(repoPath, branchName string) error {
	// Validate inputs
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	
	if err := so.validator.ValidateBranchName("branch_name", branchName); err != nil {
		return fmt.Errorf("invalid branch name: %w", err)
	}
	
	// Sanitize inputs
	repoPath = so.validator.SanitizeString(repoPath)
	branchName = so.validator.SanitizeString(branchName)
	
	return so.operations.Push(repoPath, branchName)
}

// HasChanges securely checks for changes with validation
func (so *SecureOperations) HasChanges(repoPath string) (bool, error) {
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return false, fmt.Errorf("invalid repository path: %w", err)
	}
	
	repoPath = so.validator.SanitizeString(repoPath)
	return so.operations.HasChanges(repoPath)
}

// GetCurrentBranch securely gets current branch with validation
func (so *SecureOperations) GetCurrentBranch(repoPath string) (string, error) {
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return "", fmt.Errorf("invalid repository path: %w", err)
	}
	
	repoPath = so.validator.SanitizeString(repoPath)
	return so.operations.GetCurrentBranch(repoPath)
}

// GenerateBranchName securely generates a branch name with validation
func (so *SecureOperations) GenerateBranchName(prefix string) (string, error) {
	// Validate prefix
	if err := so.validator.ValidateString("prefix", prefix, security.MaxBranchNameLength-20); err != nil {
		return "", fmt.Errorf("invalid branch prefix: %w", err)
	}
	
	// Additional validation for branch prefix
	if err := so.validateBranchPrefix(prefix); err != nil {
		return "", fmt.Errorf("branch prefix validation failed: %w", err)
	}
	
	prefix = so.validator.SanitizeString(prefix)
	branchName := so.operations.GenerateBranchName(prefix)
	
	// Validate the generated branch name
	if err := so.validator.ValidateBranchName("generated_branch", branchName); err != nil {
		return "", fmt.Errorf("generated branch name is invalid: %w", err)
	}
	
	return branchName, nil
}

// ConfigureUser securely configures git user with validation
func (so *SecureOperations) ConfigureUser(repoPath, name, email string) error {
	// Validate inputs
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return fmt.Errorf("invalid repository path: %w", err)
	}
	
	if err := so.validator.ValidateString("name", name, 100); err != nil {
		return fmt.Errorf("invalid user name: %w", err)
	}
	
	if err := so.validator.ValidateString("email", email, 100); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}
	
	// Additional email validation
	if err := so.validateEmail(email); err != nil {
		return fmt.Errorf("email validation failed: %w", err)
	}
	
	// Sanitize inputs
	repoPath = so.validator.SanitizeString(repoPath)
	name = so.validator.SanitizeString(name)
	email = so.validator.SanitizeString(email)
	
	return so.operations.ConfigureUser(repoPath, name, email)
}

// CleanupRepository securely cleans up with enhanced path validation
func (so *SecureOperations) CleanupRepository(repoPath string) error {
	if repoPath == "" {
		return fmt.Errorf("repository path cannot be empty")
	}
	
	// Enhanced validation for cleanup operations
	if err := so.validateCleanupPath(repoPath); err != nil {
		return fmt.Errorf("cleanup path validation failed: %w", err)
	}
	
	repoPath = so.validator.SanitizeString(repoPath)
	return so.operations.CleanupRepository(repoPath)
}

// GetRepositoryInfo securely gets repository information
func (so *SecureOperations) GetRepositoryInfo(repoPath string) (*Repository, error) {
	if err := so.validator.ValidateFilePath("repo_path", repoPath); err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}
	
	repoPath = so.validator.SanitizeString(repoPath)
	return so.operations.GetRepositoryInfo(repoPath)
}

// Validation helper methods

func (so *SecureOperations) validateRepositoryURL(repoURL string) error {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	
	// Only allow specific schemes
	allowedSchemes := []string{"https", "http", "git", "ssh"}
	schemeAllowed := false
	for _, scheme := range allowedSchemes {
		if parsedURL.Scheme == scheme {
			schemeAllowed = true
			break
		}
	}
	
	if !schemeAllowed {
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}
	
	// Validate hostname patterns for known git providers
	allowedHosts := []string{
		"github.com", "gitlab.com", "bitbucket.org",
		"api.github.com", "gitlab.example.com", // Add enterprise patterns as needed
	}
	
	// Allow any host that looks like a git provider (for enterprise)
	if !isAllowedGitHost(parsedURL.Host, allowedHosts) {
		return fmt.Errorf("host not allowed for git operations: %s", parsedURL.Host)
	}
	
	return nil
}

func (so *SecureOperations) validateClonePath(localPath string) error {
	// Must be an absolute path for security
	if !filepath.IsAbs(localPath) {
		return fmt.Errorf("clone path must be absolute")
	}
	
	// Must be within allowed directories
	allowedPrefixes := []string{
		"/tmp/",
		"/var/tmp/",
		"/home/", // Could be restricted further
	}
	
	pathAllowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(localPath, prefix) {
			pathAllowed = true
			break
		}
	}
	
	if !pathAllowed {
		return fmt.Errorf("clone path not in allowed directory")
	}
	
	return nil
}

func (so *SecureOperations) validateGitBranchName(branchName string) error {
	// Git-specific branch name validation
	if len(branchName) > 250 {
		return fmt.Errorf("branch name too long")
	}
	
	// Git branch naming rules
	invalidPatterns := []string{
		"..", "//", "@{", "~", "^", ":", "?", "*", "[", "\\",
	}
	
	for _, pattern := range invalidPatterns {
		if strings.Contains(branchName, pattern) {
			return fmt.Errorf("branch name contains invalid pattern: %s", pattern)
		}
	}
	
	// Cannot start/end with certain characters
	if strings.HasPrefix(branchName, ".") || strings.HasSuffix(branchName, ".") ||
	   strings.HasPrefix(branchName, "/") || strings.HasSuffix(branchName, "/") ||
	   strings.HasPrefix(branchName, "-") {
		return fmt.Errorf("branch name has invalid prefix/suffix")
	}
	
	return nil
}

func (so *SecureOperations) validateCommitOptions(options CommitOptions) error {
	if options.Message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}
	
	if err := so.validator.ValidateString("message", options.Message, security.MaxCommitMsgLength); err != nil {
		return err
	}
	
	if options.Author != "" {
		if err := so.validator.ValidateString("author", options.Author, 100); err != nil {
			return err
		}
	}
	
	if options.Email != "" {
		if err := so.validator.ValidateString("email", options.Email, 100); err != nil {
			return err
		}
		
		if err := so.validateEmail(options.Email); err != nil {
			return err
		}
	}
	
	return nil
}

func (so *SecureOperations) validateWorkingDirectory(repoPath string) error {
	// Ensure path is within expected git working directories
	cleanPath := filepath.Clean(repoPath)
	
	// Must not escape to parent directories
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal")
	}
	
	// Must be within allowed working directories
	allowedPrefixes := []string{
		"/tmp/",
		"/var/tmp/",
	}
	
	pathAllowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			pathAllowed = true
			break
		}
	}
	
	if !pathAllowed {
		return fmt.Errorf("working directory not in allowed location")
	}
	
	return nil
}

func (so *SecureOperations) validateCleanupPath(repoPath string) error {
	// Extra strict validation for cleanup operations
	if repoPath == "" || repoPath == "/" {
		return fmt.Errorf("refusing to cleanup root or empty path")
	}
	
	// Must be within safe temp directories
	safePrefixes := []string{
		"/tmp/",
		"/var/tmp/",
	}
	
	cleanPath := filepath.Clean(repoPath)
	pathSafe := false
	for _, prefix := range safePrefixes {
		if strings.HasPrefix(cleanPath, prefix) {
			pathSafe = true
			break
		}
	}
	
	if !pathSafe {
		return fmt.Errorf("cleanup path not in safe directory")
	}
	
	// Additional protection against dangerous paths
	dangerousPaths := []string{
		"/", "/bin", "/boot", "/dev", "/etc", "/home", "/lib", "/lib64",
		"/media", "/mnt", "/opt", "/proc", "/root", "/run", "/sbin",
		"/srv", "/sys", "/usr", "/var/log", "/var/lib",
	}
	
	for _, dangerous := range dangerousPaths {
		if cleanPath == dangerous || strings.HasPrefix(cleanPath, dangerous+"/") {
			return fmt.Errorf("refusing to cleanup system directory: %s", dangerous)
		}
	}
	
	return nil
}

func (so *SecureOperations) validateBranchPrefix(prefix string) error {
	// Validate branch prefix doesn't contain dangerous patterns
	if strings.ContainsAny(prefix, "~^:?*[\\") {
		return fmt.Errorf("branch prefix contains invalid characters")
	}
	
	if strings.HasPrefix(prefix, ".") || strings.HasPrefix(prefix, "/") {
		return fmt.Errorf("branch prefix has invalid start character")
	}
	
	return nil
}

func (so *SecureOperations) validateEmail(email string) error {
	// Basic email validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	
	return nil
}

// Sanitization helper methods

func (so *SecureOperations) sanitizeCommitOptions(options CommitOptions) CommitOptions {
	return CommitOptions{
		Message: so.validator.SanitizeString(options.Message),
		Author:  so.validator.SanitizeString(options.Author),
		Email:   so.validator.SanitizeString(options.Email),
	}
}

// Helper functions

func isAllowedGitHost(host string, allowedHosts []string) bool {
	// Check exact matches
	for _, allowed := range allowedHosts {
		if host == allowed {
			return true
		}
	}
	
	// Check patterns for enterprise hosts
	gitHostPatterns := []string{
		"github.", "gitlab.", "bitbucket.", "git.",
	}
	
	for _, pattern := range gitHostPatterns {
		if strings.Contains(host, pattern) {
			return true
		}
	}
	
	return false
}