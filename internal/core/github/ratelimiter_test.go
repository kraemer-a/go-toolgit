package github

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v66/github"
)

func TestGitHubRateLimiter_Creation(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	
	if grl.waitForReset {
		t.Error("Expected waitForReset to be false")
	}
	
	info := grl.GetRateLimitInfo()
	if info.Core.Limit != 5000 {
		t.Errorf("Expected core limit 5000, got %d", info.Core.Limit)
	}
	if info.Search.Limit != 30 {
		t.Errorf("Expected search limit 30, got %d", info.Search.Limit)
	}
}

func TestGitHubRateLimiter_CheckRateLimit_Allowed(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	ctx := context.Background()
	
	// Should allow initial requests
	err := grl.CheckRateLimit(ctx, false)
	if err != nil {
		t.Errorf("Expected rate limit check to pass: %v", err)
	}
	
	// Should allow search requests
	err = grl.CheckRateLimit(ctx, true)
	if err != nil {
		t.Errorf("Expected search rate limit check to pass: %v", err)
	}
}

func TestGitHubRateLimiter_CheckRateLimit_Exceeded(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	ctx := context.Background()
	
	// Simulate exhausted rate limit
	grl.mu.Lock()
	grl.coreRate.Remaining = 0
	grl.coreRate.Reset = github.Timestamp{Time: time.Now().Add(time.Hour)}
	grl.mu.Unlock()
	
	// Should return error when rate limited
	err := grl.CheckRateLimit(ctx, false)
	if err == nil {
		t.Error("Expected rate limit error")
	}
	
	ghErr, ok := err.(*GitHubRateLimitError)
	if !ok {
		t.Errorf("Expected GitHubRateLimitError, got %T", err)
	}
	
	if ghErr.Type != "core" {
		t.Errorf("Expected core rate limit error, got %s", ghErr.Type)
	}
}

func TestGitHubRateLimiter_UpdateFromResponse(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	
	// Create mock response with rate limit info
	resp := &github.Response{
		Rate: github.Rate{
			Limit:     4000,
			Remaining: 3500,
			Reset:     github.Timestamp{Time: time.Now().Add(30 * time.Minute)},
		},
	}
	
	// Update from response
	grl.UpdateFromResponse(resp, false)
	
	info := grl.GetRateLimitInfo()
	if info.Core.Limit != 4000 {
		t.Errorf("Expected core limit 4000, got %d", info.Core.Limit)
	}
	if info.Core.Remaining != 3500 {
		t.Errorf("Expected core remaining 3500, got %d", info.Core.Remaining)
	}
}

func TestGitHubRateLimiter_SearchTracking(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	
	// Reset search tracking
	grl.mu.Lock()
	grl.searchCount = 0
	grl.lastSearchTime = time.Now()
	grl.mu.Unlock()
	
	// Simulate multiple search requests
	resp := &github.Response{
		Rate: github.Rate{
			Limit:     5000,
			Remaining: 4999,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Hour)},
		},
	}
	
	for i := 0; i < 5; i++ {
		grl.UpdateFromResponse(resp, true)
	}
	
	info := grl.GetRateLimitInfo()
	if info.Search.Remaining != 25 { // 30 - 5
		t.Errorf("Expected search remaining 25, got %d", info.Search.Remaining)
	}
}

func TestGitHubRateLimiter_WaitForReset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test")
	}
	
	grl := NewGitHubRateLimiter(true) // Enable wait for reset
	ctx := context.Background()
	
	// Simulate exhausted rate limit with short reset time
	grl.mu.Lock()
	grl.searchRate.Remaining = 0
	grl.searchRate.Reset = github.Timestamp{Time: time.Now().Add(100 * time.Millisecond)}
	grl.mu.Unlock()
	
	start := time.Now()
	err := grl.CheckRateLimit(ctx, true)
	elapsed := time.Since(start)
	
	if err != nil {
		t.Errorf("Expected wait to succeed, got error: %v", err)
	}
	
	// Should have waited at least 100ms
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected to wait at least 100ms, waited %v", elapsed)
	}
}

func TestGitHubRateLimiter_WaitForReset_ContextCancellation(t *testing.T) {
	grl := NewGitHubRateLimiter(true)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	// Simulate exhausted rate limit with long reset time
	grl.mu.Lock()
	grl.coreRate.Remaining = 0
	grl.coreRate.Reset = github.Timestamp{Time: time.Now().Add(time.Hour)}
	grl.mu.Unlock()
	
	err := grl.CheckRateLimit(ctx, false)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context error, got: %v", err)
	}
}

func TestGitHubRateLimiter_ResetTimePassed(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	ctx := context.Background()
	
	// Simulate exhausted rate limit but reset time has passed
	grl.mu.Lock()
	grl.coreRate.Remaining = 0
	grl.coreRate.Reset = github.Timestamp{Time: time.Now().Add(-time.Minute)} // Past time
	grl.mu.Unlock()
	
	// Should allow request since reset time has passed
	err := grl.CheckRateLimit(ctx, false)
	if err != nil {
		t.Errorf("Expected rate limit check to pass after reset time: %v", err)
	}
}

func TestIsRateLimitError(t *testing.T) {
	// Test GitHubRateLimitError
	ghErr := &GitHubRateLimitError{
		Type:      "search",
		ResetTime: time.Now().Add(time.Hour),
		Message:   "rate limited",
	}
	
	if !IsRateLimitError(ghErr) {
		t.Error("Expected GitHubRateLimitError to be recognized")
	}
	
	// Test github.RateLimitError
	githubErr := &github.RateLimitError{
		Rate: github.Rate{
			Limit:     30,
			Remaining: 0,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Minute)},
		},
		Message: "API rate limit exceeded",
	}
	
	if !IsRateLimitError(githubErr) {
		t.Error("Expected github.RateLimitError to be recognized")
	}
	
	// Test non-rate-limit error
	normalErr := fmt.Errorf("normal error")
	if IsRateLimitError(normalErr) {
		t.Error("Expected normal error not to be recognized as rate limit error")
	}
}

func TestGetResetTime(t *testing.T) {
	expectedTime := time.Now().Add(time.Hour)
	
	// Test GitHubRateLimitError
	ghErr := &GitHubRateLimitError{
		Type:      "core",
		ResetTime: expectedTime,
		Message:   "rate limited",
	}
	
	resetTime, ok := GetResetTime(ghErr)
	if !ok {
		t.Error("Expected to get reset time from GitHubRateLimitError")
	}
	if !resetTime.Equal(expectedTime) {
		t.Errorf("Expected reset time %v, got %v", expectedTime, resetTime)
	}
	
	// Test github.RateLimitError
	githubErr := &github.RateLimitError{
		Rate: github.Rate{
			Reset: github.Timestamp{Time: expectedTime},
		},
	}
	
	resetTime, ok = GetResetTime(githubErr)
	if !ok {
		t.Error("Expected to get reset time from github.RateLimitError")
	}
	if !resetTime.Equal(expectedTime) {
		t.Errorf("Expected reset time %v, got %v", expectedTime, resetTime)
	}
	
	// Test non-rate-limit error
	normalErr := fmt.Errorf("normal error")
	_, ok = GetResetTime(normalErr)
	if ok {
		t.Error("Expected not to get reset time from normal error")
	}
}

func TestGitHubRateLimiter_SearchRateReset(t *testing.T) {
	grl := NewGitHubRateLimiter(false)
	
	// Set search count and time in the past
	grl.mu.Lock()
	grl.searchCount = 25
	grl.lastSearchTime = time.Now().Add(-2 * time.Minute)
	grl.mu.Unlock()
	
	// Update with a search request - should reset counter
	resp := &github.Response{
		Rate: github.Rate{
			Limit:     5000,
			Remaining: 4999,
		},
	}
	
	grl.UpdateFromResponse(resp, true)
	
	info := grl.GetRateLimitInfo()
	// Should have reset to 29 (30 - 1 for the current request)
	if info.Search.Remaining != 29 {
		t.Errorf("Expected search remaining 29 after reset, got %d", info.Search.Remaining)
	}
}

// Benchmark tests
func BenchmarkGitHubRateLimiter_CheckRateLimit(b *testing.B) {
	grl := NewGitHubRateLimiter(false)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		grl.CheckRateLimit(ctx, false)
	}
}

func BenchmarkGitHubRateLimiter_UpdateFromResponse(b *testing.B) {
	grl := NewGitHubRateLimiter(false)
	resp := &github.Response{
		Rate: github.Rate{
			Limit:     5000,
			Remaining: 4999,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Hour)},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		grl.UpdateFromResponse(resp, false)
	}
}