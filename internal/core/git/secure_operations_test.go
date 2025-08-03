package git

import (
	"strings"
	"testing"
)

func TestSecureOperations_ValidatesInputs(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that malicious repository URLs are rejected
	err = secureOps.CloneRepository("https://github.com/user/repo.git; rm -rf /", "/tmp/test")
	if err == nil {
		t.Error("Expected secure operations to reject malicious repository URL")
	}
	
	if !strings.Contains(err.Error(), "invalid repository URL") {
		t.Errorf("Expected URL validation error, got: %v", err)
	}
}

func TestSecureOperations_ValidatesBranchNames(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that malicious branch names are rejected
	err = secureOps.CreateBranch("/tmp/test-repo", "feature; rm -rf /")
	if err == nil {
		t.Error("Expected secure operations to reject malicious branch name")
	}
	
	if !strings.Contains(err.Error(), "invalid branch name") {
		t.Errorf("Expected branch validation error, got: %v", err)
	}
}

func TestSecureOperations_ValidatesCommitOptions(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that malicious commit options are rejected
	maliciousCommit := CommitOptions{
		Message: "commit; rm -rf /",
		Author:  "<script>alert('xss')</script>",
		Email:   "test@example.com",
	}
	
	err = secureOps.Commit("/tmp/test-repo", maliciousCommit)
	if err == nil {
		t.Error("Expected secure operations to reject malicious commit options")
	}
	
	if !strings.Contains(err.Error(), "invalid commit options") {
		t.Errorf("Expected commit validation error, got: %v", err)
	}
}

func TestSecureOperations_ValidatesToken(t *testing.T) {
	// Test that malicious tokens are rejected
	_, err := NewSecureOperationsWithToken("token; rm -rf /")
	if err == nil {
		t.Error("Expected secure operations to reject malicious token")
	}
	
	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("Expected token validation error, got: %v", err)
	}
}

func TestSecureOperations_ValidatesFilePaths(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that dangerous file paths are rejected
	dangerousPaths := []string{
		"../../../etc/passwd",
		"/etc/shadow",
		"C:\\Windows\\System32",
		"/tmp/test; rm -rf /",
	}
	
	for _, path := range dangerousPaths {
		err = secureOps.AddAllChanges(path)
		if err == nil {
			t.Errorf("Expected secure operations to reject dangerous path: %s", path)
		}
	}
}

func TestSecureOperations_ValidatesCleanupPaths(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that dangerous cleanup paths are rejected
	dangerousPaths := []string{
		"",
		"/",
		"/etc",
		"/home",
		"C:\\Windows",
		"../../../etc",
	}
	
	for _, path := range dangerousPaths {
		err = secureOps.CleanupRepository(path)
		if err == nil {
			t.Errorf("Expected secure operations to reject dangerous cleanup path: %s", path)
		}
	}
}

func TestSecureOperations_GeneratesBranchNameSecurely(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that malicious branch prefixes are rejected
	maliciousPrefixes := []string{
		"feature; rm -rf /",
		"<script>alert('xss')</script>",
		"../../../etc/passwd",
		strings.Repeat("A", 1000),
	}
	
	for _, prefix := range maliciousPrefixes {
		_, err = secureOps.GenerateBranchName(prefix)
		if err == nil {
			t.Errorf("Expected secure operations to reject malicious branch prefix: %s", prefix)
		}
	}
	
	// Test that valid prefixes work
	branchName, err := secureOps.GenerateBranchName("feature")
	if err != nil {
		t.Errorf("Expected valid branch prefix to work: %v", err)
	}
	
	if !strings.HasPrefix(branchName, "feature-") {
		t.Errorf("Expected branch name to start with 'feature-', got: %s", branchName)
	}
}

func TestSecureOperations_ValidatesUserConfig(t *testing.T) {
	secureOps, err := NewSecureOperations()
	if err != nil {
		t.Fatalf("Failed to create secure operations: %v", err)
	}
	
	// Test that malicious user config is rejected
	err = secureOps.ConfigureUser("/tmp/test", "user; rm -rf /", "invalid-email")
	if err == nil {
		t.Error("Expected secure operations to reject malicious user config")
	}
	
	// Test that invalid email is rejected
	err = secureOps.ConfigureUser("/tmp/test", "Valid User", "not-an-email")
	if err == nil {
		t.Error("Expected secure operations to reject invalid email")
	}
	
	if !strings.Contains(err.Error(), "email validation failed") {
		t.Errorf("Expected email validation error, got: %v", err)
	}
}

func TestSecureOperations_AllowsValidInputs(t *testing.T) {
	// Test that valid inputs are accepted
	validToken := "ghp_valid_test_token_123456789"
	secureOps, err := NewSecureOperationsWithToken(validToken)
	if err != nil {
		t.Errorf("Expected valid token to be accepted: %v", err)
	}
	
	// Test valid branch name generation
	branchName, err := secureOps.GenerateBranchName("feature")
	if err != nil {
		t.Errorf("Expected valid branch prefix to work: %v", err)
	}
	
	if len(branchName) == 0 {
		t.Error("Expected non-empty branch name")
	}
}

// Benchmark tests to measure security overhead
func BenchmarkSecureOperationsCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSecureOperations()
	}
}

func BenchmarkSecureVsUnsafeBranchGeneration(b *testing.B) {
	secureOps, _ := NewSecureOperations()
	unsafeOps, _ := NewOperations()
	
	b.Run("Secure", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			secureOps.GenerateBranchName("feature")
		}
	})
	
	b.Run("Unsafe", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			unsafeOps.GenerateBranchName("feature")
		}
	})
}