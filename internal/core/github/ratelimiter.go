package github

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-github/v66/github"
)

// GitHubRateLimiter manages rate limits specific to GitHub API
type GitHubRateLimiter struct {
	mu              sync.RWMutex
	coreRate        *github.Rate
	searchRate      *github.Rate
	lastSearchTime  time.Time
	searchCount     int
	waitForReset    bool
}

// NewGitHubRateLimiter creates a new GitHub-specific rate limiter
func NewGitHubRateLimiter(waitForReset bool) *GitHubRateLimiter {
	return &GitHubRateLimiter{
		waitForReset: waitForReset,
		coreRate: &github.Rate{
			Limit:     5000,
			Remaining: 5000,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Hour)},
		},
		searchRate: &github.Rate{
			Limit:     30,
			Remaining: 30,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Minute)},
		},
	}
}

// UpdateFromResponse updates rate limit information from GitHub API response
func (grl *GitHubRateLimiter) UpdateFromResponse(resp *github.Response, isSearch bool) {
	if resp == nil {
		return
	}

	grl.mu.Lock()
	defer grl.mu.Unlock()

	// Update core rate limit from response
	if resp.Rate.Limit > 0 {
		grl.coreRate = &resp.Rate
	}

	// Track search requests separately
	if isSearch {
		now := time.Now()
		
		// Reset search counter if minute has passed
		if now.Sub(grl.lastSearchTime) >= time.Minute {
			grl.searchCount = 0
			grl.lastSearchTime = now
		}
		grl.searchCount++
		
		// Update search rate based on our tracking
		grl.searchRate.Remaining = 30 - grl.searchCount
		if grl.searchRate.Remaining < 0 {
			grl.searchRate.Remaining = 0
		}
		grl.searchRate.Reset = github.Timestamp{Time: grl.lastSearchTime.Add(time.Minute)}
	}
}

// UpdateFromRateLimitResponse updates rate limits from a dedicated rate limit API call
func (grl *GitHubRateLimiter) UpdateFromRateLimitResponse(rateLimits *github.RateLimits) {
	if rateLimits == nil {
		return
	}
	
	grl.mu.Lock()
	defer grl.mu.Unlock()
	
	// Update core rate limit
	if rateLimits.Core != nil {
		grl.coreRate = &github.Rate{
			Limit:     rateLimits.Core.Limit,
			Remaining: rateLimits.Core.Remaining,
			Reset:     rateLimits.Core.Reset,
		}
	}
	
	// Update search rate limit
	if rateLimits.Search != nil {
		grl.searchRate = &github.Rate{
			Limit:     rateLimits.Search.Limit,
			Remaining: rateLimits.Search.Remaining,
			Reset:     rateLimits.Search.Reset,
		}
		// Reset our search tracking since we have fresh data
		grl.searchCount = 30 - rateLimits.Search.Remaining
		grl.lastSearchTime = time.Now()
	}
}

// CheckRateLimit checks if we can make a request of the given type
func (grl *GitHubRateLimiter) CheckRateLimit(ctx context.Context, isSearch bool) error {
	grl.mu.RLock()
	
	var rate *github.Rate
	var limitType string
	
	if isSearch {
		rate = grl.searchRate
		limitType = "search"
	} else {
		rate = grl.coreRate
		limitType = "core"
	}
	
	remaining := rate.Remaining
	resetTime := rate.Reset.Time
	grl.mu.RUnlock()

	// If we have remaining requests, allow it
	if remaining > 0 {
		return nil
	}

	// Check if reset time has passed
	if time.Now().After(resetTime) {
		// Reset time has passed, we should be good
		return nil
	}

	// We're rate limited
	waitDuration := time.Until(resetTime)
	
	if !grl.waitForReset {
		return &GitHubRateLimitError{
			Type:      limitType,
			ResetTime: resetTime,
			Message:   fmt.Sprintf("GitHub %s API rate limit exceeded. Reset at %s", limitType, resetTime.Format(time.RFC3339)),
		}
	}

	// Wait for rate limit reset if configured to do so
	select {
	case <-time.After(waitDuration):
		// Rate limit should be reset now
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for rate limit reset: %w", ctx.Err())
	}
}

// GetRateLimitInfo returns current rate limit information
func (grl *GitHubRateLimiter) GetRateLimitInfo() RateLimitInfo {
	grl.mu.RLock()
	defer grl.mu.RUnlock()

	return RateLimitInfo{
		Core: RateInfo{
			Limit:     grl.coreRate.Limit,
			Remaining: grl.coreRate.Remaining,
			Reset:     grl.coreRate.Reset.Time,
		},
		Search: RateInfo{
			Limit:     grl.searchRate.Limit,
			Remaining: grl.searchRate.Remaining,
			Reset:     grl.searchRate.Reset.Time,
		},
	}
}

// ShouldWaitForReset returns whether the rate limiter is configured to wait
func (grl *GitHubRateLimiter) ShouldWaitForReset() bool {
	return grl.waitForReset
}

// SetWaitForReset configures whether to wait for rate limit reset
func (grl *GitHubRateLimiter) SetWaitForReset(wait bool) {
	grl.mu.Lock()
	defer grl.mu.Unlock()
	grl.waitForReset = wait
}

// RateLimitInfo contains rate limit information for different API categories
type RateLimitInfo struct {
	Core   RateInfo `json:"core"`
	Search RateInfo `json:"search"`
}

// RateInfo contains rate limit details for a specific API category
type RateInfo struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// GitHubRateLimitError represents a GitHub rate limit error
type GitHubRateLimitError struct {
	Type      string    // "core" or "search"
	ResetTime time.Time
	Message   string
}

func (e *GitHubRateLimitError) Error() string {
	return e.Message
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	if _, ok := err.(*GitHubRateLimitError); ok {
		return true
	}
	
	// Also check for go-github rate limit errors
	if _, ok := err.(*github.RateLimitError); ok {
		return true
	}
	
	return false
}

// GetResetTime extracts the reset time from a rate limit error
func GetResetTime(err error) (time.Time, bool) {
	if ghErr, ok := err.(*GitHubRateLimitError); ok {
		return ghErr.ResetTime, true
	}
	
	if rlErr, ok := err.(*github.RateLimitError); ok {
		return rlErr.Rate.Reset.Time, true
	}
	
	return time.Time{}, false
}