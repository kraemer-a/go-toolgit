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

// CloneRepositoryWithBasicAuth clones a repository using username/password authentication (for Bitbucket)
func (m *MemoryOperations) CloneRepositoryWithBasicAuth(ctx context.Context, repoURL, fullName, username, password string) (*MemoryRepository, error) {
	startTime := time.Now()

	storage := memory.NewStorage()
	fs := memfs.New()

	auth := &http.BasicAuth{
		Username: username, // Bitbucket username
		Password: password, // Bitbucket password/app password
	}

	repo, err := git.Clone(storage, fs, &git.CloneOptions{
		URL:  repoURL,
		Auth: auth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s: %w", fullName, err)
	}

	cloneDuration := time.Since(startTime)
	log.Printf("[INFO] Successfully cloned repository %s using basic auth in %v", fullName, cloneDuration)

	return &MemoryRepository{
		repo:     repo,
		fs:       fs,
		auth:     auth,
		repoURL:  repoURL,
		fullName: fullName,
	}, nil
}

// GetDefaultBranch returns the default branch of the repository (main or master)
func (mr *MemoryRepository) GetDefaultBranch() (string, error) {
	// Try to get HEAD reference
	head, err := mr.repo.Head()
	if err == nil {
		// Extract branch name from HEAD
		branchName := head.Name().Short()
		if branchName != "" {
			return branchName, nil
		}
	}

	// If HEAD doesn't work, check for common default branches
	refs, err := mr.repo.References()
	if err != nil {
		return "", fmt.Errorf("failed to get references: %w", err)
	}

	var mainExists, masterExists bool
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() {
			branchName := ref.Name().Short()
			if branchName == "main" {
				mainExists = true
			} else if branchName == "master" {
				masterExists = true
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to iterate references: %w", err)
	}

	// Prefer main over master
	if mainExists {
		return "main", nil
	}
	if masterExists {
		return "master", nil
	}

	return "", fmt.Errorf("no default branch found (neither main nor master)")
}

// CommitAndPushToDefault commits changes and pushes directly to the default branch
func (mr *MemoryRepository) CommitAndPushToDefault(ctx context.Context, commitMessage string) error {
	// Get the default branch
	defaultBranch, err := mr.GetDefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to get default branch: %w", err)
	}

	// Get the working tree
	worktree, err := mr.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Note: We don't need to checkout the default branch because after clone,
	// the repository is already on the default branch and files have been modified in place

	// Add all changes to staging area
	_, err = worktree.Add(".")
	if err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Commit the changes
	_, err = worktree.Commit(commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "GitHub Replace Tool",
			Email: "go-toolgit@automated.tool",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Push directly to the default branch
	err = mr.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", defaultBranch, defaultBranch)),
		},
		Auth: mr.auth,
	})
	if err != nil {
		return fmt.Errorf("failed to push to %s branch: %w", defaultBranch, err)
	}

	log.Printf("[INFO] Successfully pushed changes directly to %s branch", defaultBranch)
	return nil
}

// CloneRepositoryWithMirror clones a repository with all branches and tags using basic auth
func (m *MemoryOperations) CloneRepositoryWithMirror(ctx context.Context, repoURL, fullName, username, password string) (*MemoryRepository, error) {
	startTime := time.Now()

	storage := memory.NewStorage()
	fs := memfs.New()

	auth := &http.BasicAuth{
		Username: username, // Bitbucket username
		Password: password, // Bitbucket password/app password
	}

	// Clone with working directory to ensure file contents are loaded properly
	repo, err := git.Clone(storage, fs, &git.CloneOptions{
		URL:          repoURL,
		Auth:         auth,
		SingleBranch: false,
		NoCheckout:   false, // CHANGED: Create working directory to load file contents
		Tags:         git.AllTags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s: %w", fullName, err)
	}

	// Fetch all remote branches to ensure we have everything
	err = repo.Fetch(&git.FetchOptions{
		Auth: auth,
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/remotes/origin/*",
			"refs/tags/*:refs/tags/*",
		},
		Tags: git.AllTags,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Printf("[WARN] Failed to fetch additional refs for %s: %v", fullName, err)
	}

	// Convert remote branches to local branches
	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to get references: %w", err)
	}

	remoteBranchCount := 0
	localBranchCount := 0

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		refStr := ref.Name().String()
		log.Printf("[DEBUG] Found reference: %s (hash: %s)", refStr, ref.Hash().String()[:8])

		if ref.Name().IsRemote() {
			remoteBranchCount++
			if strings.HasPrefix(refStr, "refs/remotes/origin/") && !strings.HasSuffix(refStr, "/HEAD") {
				branchName := strings.TrimPrefix(refStr, "refs/remotes/origin/")

				// Check if local branch already exists and compare hashes
				existingRef, err := repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
				if err == nil {
					log.Printf("[DEBUG] Local branch %s already exists (hash: %s), remote hash: %s", branchName, existingRef.Hash().String()[:8], ref.Hash().String()[:8])
					if existingRef.Hash() != ref.Hash() {
						log.Printf("[WARN] Local and remote %s have different commits! Updating local to match remote.", branchName)
					}
				}

				localRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), ref.Hash())
				err = repo.Storer.SetReference(localRef)
				if err != nil {
					log.Printf("[WARN] Failed to create local branch %s: %v", branchName, err)
				} else {
					log.Printf("[INFO] Created/updated local branch %s from remote %s (hash: %s)", branchName, refStr, ref.Hash().String()[:8])
					localBranchCount++
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert remote branches: %w", err)
	}

	log.Printf("[INFO] Found %d remote branches, created %d local branches", remoteBranchCount, localBranchCount)

	// Verify local branches were created by listing all references again
	allRefs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to verify references: %w", err)
	}

	finalBranchCount := 0
	allRefs.ForEach(func(ref *plumbing.Reference) error {
		refStr := ref.Name().String()
		if ref.Name().IsBranch() {
			finalBranchCount++
			log.Printf("[DEBUG] Final local branch: %s (hash: %s)", refStr, ref.Hash().String()[:8])
		}
		return nil
	})

	log.Printf("[INFO] Repository has %d local branches after conversion", finalBranchCount)

	cloneDuration := time.Since(startTime)
	log.Printf("[INFO] Successfully cloned repository %s with all branches in %v", fullName, cloneDuration)

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

// PushToRemote pushes the branch to a different remote repository
func (mr *MemoryRepository) PushToRemote(ctx context.Context, remoteURL, branchName, token string) error {
	// Create authentication for the new remote
	auth := &http.BasicAuth{
		Username: "git", // Can be anything for GitHub PAT
		Password: token,
	}

	// Add the new remote
	_, err := mr.repo.CreateRemote(&config.RemoteConfig{
		Name: "migration-target",
		URLs: []string{remoteURL},
	})
	if err != nil {
		// If remote already exists, get it
		_, err = mr.repo.Remote("migration-target")
		if err != nil {
			return fmt.Errorf("failed to create or get migration-target remote: %w", err)
		}
	}

	// Push the branch to the new remote
	err = mr.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "migration-target",
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/heads/" + branchName + ":refs/heads/" + branchName),
		},
		Auth: auth,
	})
	if err != nil {
		return fmt.Errorf("failed to push branch %s to migration target: %w", branchName, err)
	}

	return nil
}

// PushAllBranchesToRemote pushes all branches to a different remote repository
func (mr *MemoryRepository) PushAllBranchesToRemote(ctx context.Context, remoteURL, token string) error {
	// Create authentication for the new remote
	auth := &http.BasicAuth{
		Username: "git", // Can be anything for GitHub PAT
		Password: token,
	}

	// Add the new remote
	_, err := mr.repo.CreateRemote(&config.RemoteConfig{
		Name: "migration-target",
		URLs: []string{remoteURL},
	})
	if err != nil {
		// If remote already exists, that's fine
		if err.Error() != "remote already exists" {
			return fmt.Errorf("failed to create migration-target remote: %w", err)
		}
	}

	// Get all branches
	refs, err := mr.repo.References()
	if err != nil {
		return fmt.Errorf("failed to get references: %w", err)
	}

	var refSpecs []config.RefSpec
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsBranch() {
			refSpec := config.RefSpec(ref.Name() + ":" + ref.Name())
			refSpecs = append(refSpecs, refSpec)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to iterate references: %w", err)
	}

	// Push all branches to the new remote
	err = mr.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "migration-target",
		RefSpecs:   refSpecs,
		Auth:       auth,
	})
	if err != nil {
		return fmt.Errorf("failed to push all branches to migration target: %w", err)
	}

	return nil
}

// PushAllReferencesToRemote pushes all references (branches, tags, etc.) to a different remote repository (mirror push)
func (mr *MemoryRepository) PushAllReferencesToRemote(ctx context.Context, remoteURL, token string) error {
	return mr.PushAllReferencesToRemoteWithOptions(ctx, remoteURL, token, true)
}

// PushAllReferencesToRemoteWithOptions pushes all references with master→main transformation option
func (mr *MemoryRepository) PushAllReferencesToRemoteWithOptions(ctx context.Context, remoteURL, token string, transformMasterToMain bool) error {
	// Create authentication for the new remote
	auth := &http.BasicAuth{
		Username: "git", // Can be anything for GitHub PAT
		Password: token,
	}

	// Add the new remote
	_, err := mr.repo.CreateRemote(&config.RemoteConfig{
		Name: "migration-target",
		URLs: []string{remoteURL},
	})
	if err != nil {
		// If remote already exists, that's fine
		if err.Error() != "remote already exists" {
			return fmt.Errorf("failed to create migration-target remote: %w", err)
		}
	}

	// Get all references
	refs, err := mr.repo.References()
	if err != nil {
		return fmt.Errorf("failed to get references: %w", err)
	}

	var refSpecs []config.RefSpec
	var masterTransformed bool

	// Check if a master branch exists
	hasMasterBranch := false
	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().String() == "refs/heads/master" {
			hasMasterBranch = true
		}
		return nil
	})

	// Create a fresh references iterator for the actual push operation
	pushRefs, err := mr.repo.References()
	if err != nil {
		return fmt.Errorf("failed to get references for push: %w", err)
	}

	err = pushRefs.ForEach(func(ref *plumbing.Reference) error {
		refName := ref.Name()

		// Handle branches, tags, and notes
		if refName.IsBranch() {
			branchName := strings.TrimPrefix(refName.String(), "refs/heads/")

			// Handle master→main transformation for branches
			if transformMasterToMain && branchName == "master" {
				// Transform master branch to main
				targetRef := "refs/heads/main"
				refSpec := config.RefSpec(refName + ":" + plumbing.ReferenceName(targetRef))
				refSpecs = append(refSpecs, refSpec)
				masterTransformed = true
				log.Printf("[INFO] Transforming branch master to main during migration")
			} else if transformMasterToMain && branchName == "main" && hasMasterBranch {
				// Only skip existing main branch if we actually have a master to transform
				log.Printf("[INFO] Skipping existing main branch (master→main transformation will replace it)")
				return nil
			} else {
				// Direct mapping for other branches (including main when no master exists)
				refSpec := config.RefSpec(refName + ":" + refName)
				refSpecs = append(refSpecs, refSpec)
				log.Printf("[INFO] Pushing branch %s", branchName)
			}
		} else if refName.IsTag() || refName.IsNote() {
			// Direct mapping for tags and notes
			refSpec := config.RefSpec(refName + ":" + refName)
			refSpecs = append(refSpecs, refSpec)
			if refName.IsTag() {
				log.Printf("[INFO] Pushing tag %s", strings.TrimPrefix(refName.String(), "refs/tags/"))
			}
		} else if refName.IsRemote() {
			// Skip remote refs - we should have local branches now
			return nil
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to iterate references: %w", err)
	}

	if len(refSpecs) == 0 {
		return fmt.Errorf("no references found to push")
	}

	if masterTransformed {
		log.Printf("[INFO] Pushing %d references to migration target (master→main transformation applied)", len(refSpecs))
	} else {
		log.Printf("[INFO] Pushing %d references to migration target", len(refSpecs))
	}

	// Push all references to the new remote with force to handle any conflicts
	err = mr.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "migration-target",
		RefSpecs:   refSpecs,
		Auth:       auth,
		Force:      true, // Force push to ensure complete migration
	})
	if err != nil {
		return fmt.Errorf("failed to push all references to migration target: %w", err)
	}

	return nil
}

// MigrationResult contains information about the push operation
type MigrationPushResult struct {
	Success           bool
	MasterTransformed bool
	ReferencesCount   int
	Error             error
}

// PushAllReferencesToRemoteWithResult pushes all references and returns detailed result information
func (mr *MemoryRepository) PushAllReferencesToRemoteWithResult(ctx context.Context, remoteURL, token string, transformMasterToMain bool) *MigrationPushResult {
	result := &MigrationPushResult{
		Success: false,
	}

	// Create authentication for the new remote
	auth := &http.BasicAuth{
		Username: "git", // Can be anything for GitHub PAT
		Password: token,
	}

	// Add the new remote
	_, err := mr.repo.CreateRemote(&config.RemoteConfig{
		Name: "migration-target",
		URLs: []string{remoteURL},
	})
	if err != nil {
		// If remote already exists, that's fine
		if err.Error() != "remote already exists" {
			result.Error = fmt.Errorf("failed to create migration-target remote: %w", err)
			return result
		}
	}

	// Get all references
	refs, err := mr.repo.References()
	if err != nil {
		result.Error = fmt.Errorf("failed to get references: %w", err)
		return result
	}

	var refSpecs []config.RefSpec
	var masterTransformed bool

	// Check if a master branch exists and debug all references
	hasMasterBranch := false
	branchCount := 0
	tagCount := 0
	remoteCount := 0

	refs.ForEach(func(ref *plumbing.Reference) error {
		refStr := ref.Name().String()
		log.Printf("[DEBUG] Push: Found reference: %s", refStr)

		if ref.Name().String() == "refs/heads/master" {
			hasMasterBranch = true
		}
		if ref.Name().IsBranch() {
			branchCount++
		} else if ref.Name().IsTag() {
			tagCount++
		} else if ref.Name().IsRemote() {
			remoteCount++
		}
		return nil
	})

	log.Printf("[INFO] Push: Found %d branches, %d tags, %d remotes. Master branch exists: %t", branchCount, tagCount, remoteCount, hasMasterBranch)

	// Create a fresh references iterator for the actual push operation
	pushRefs, err := mr.repo.References()
	if err != nil {
		result.Error = fmt.Errorf("failed to get references for push: %w", err)
		return result
	}

	err = pushRefs.ForEach(func(ref *plumbing.Reference) error {
		refName := ref.Name()

		// Handle branches, tags, and notes
		if refName.IsBranch() {
			branchName := strings.TrimPrefix(refName.String(), "refs/heads/")

			// Handle master→main transformation for branches
			if transformMasterToMain && branchName == "master" {
				// Transform master branch to main
				targetRef := "refs/heads/main"
				refSpec := config.RefSpec(refName + ":" + plumbing.ReferenceName(targetRef))
				refSpecs = append(refSpecs, refSpec)
				masterTransformed = true
				log.Printf("[INFO] Transforming branch master to main during push")
			} else if transformMasterToMain && branchName == "main" && hasMasterBranch {
				// Only skip existing main branch if we actually have a master to transform
				log.Printf("[INFO] Skipping existing main branch (master→main transformation will replace it)")
				return nil
			} else {
				// Direct mapping for other branches (including main when no master exists)
				refSpec := config.RefSpec(refName + ":" + refName)
				refSpecs = append(refSpecs, refSpec)
				log.Printf("[INFO] Pushing branch %s", branchName)
			}
		} else if refName.IsTag() || refName.IsNote() {
			// Direct mapping for tags and notes
			refSpec := config.RefSpec(refName + ":" + refName)
			refSpecs = append(refSpecs, refSpec)
			if refName.IsTag() {
				log.Printf("[INFO] Pushing tag %s", strings.TrimPrefix(refName.String(), "refs/tags/"))
			}
		} else if refName.IsRemote() {
			// Skip remote refs - we should have local branches now
			return nil
		}
		return nil
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to iterate references: %w", err)
		return result
	}

	if len(refSpecs) == 0 {
		result.Error = fmt.Errorf("no references found to push")
		return result
	}

	result.ReferencesCount = len(refSpecs)
	result.MasterTransformed = masterTransformed

	if masterTransformed {
		log.Printf("[INFO] Pushing %d references to migration target (master→main transformation applied)", len(refSpecs))
	} else {
		log.Printf("[INFO] Pushing %d references to migration target", len(refSpecs))
	}

	// Push all references to the new remote with force to handle any conflicts
	err = mr.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "migration-target",
		RefSpecs:   refSpecs,
		Auth:       auth,
		Force:      true, // Force push to ensure complete migration
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to push all references to migration target: %w", err)
		return result
	}

	result.Success = true
	return result
}

// DetectDefaultBranch detects the default branch of the repository
func (mr *MemoryRepository) DetectDefaultBranch() (string, error) {
	// Try to get the HEAD reference to determine default branch
	head, err := mr.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not pointing to a branch")
	}

	// Extract branch name from reference (refs/heads/branch-name -> branch-name)
	branchName := head.Name().Short()
	return branchName, nil
}

// HasMasterBranch checks if the repository has a master branch
func (mr *MemoryRepository) HasMasterBranch() bool {
	refs, err := mr.repo.References()
	if err != nil {
		return false
	}

	hasMaster := false
	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().String() == "refs/heads/master" {
			hasMaster = true
		}
		return nil
	})

	return hasMaster
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
