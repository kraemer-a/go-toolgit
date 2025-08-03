package security

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter provides rate limiting functionality to prevent abuse
type RateLimiter struct {
	tokens    chan struct{}
	ticker    *time.Ticker
	maxTokens int
	window    time.Duration
	mu        sync.RWMutex
	stopped   bool
}

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
	RequestsPerWindow int           // Maximum requests per window
	Window           time.Duration // Time window for rate limiting
	BurstLimit       int           // Maximum burst requests allowed
}

// NewRateLimiter creates a new rate limiter with token bucket algorithm
func NewRateLimiter(config RateLimitConfig) (*RateLimiter, error) {
	if config.RequestsPerWindow <= 0 {
		return nil, fmt.Errorf("requests per window must be positive")
	}
	if config.Window <= 0 {
		return nil, fmt.Errorf("window duration must be positive")
	}
	if config.BurstLimit <= 0 {
		config.BurstLimit = config.RequestsPerWindow // Default to same as rate limit
	}

	// Create token bucket with burst capacity
	tokens := make(chan struct{}, config.BurstLimit)
	
	// Fill initial tokens
	for i := 0; i < config.BurstLimit; i++ {
		tokens <- struct{}{}
	}

	// Calculate refill interval
	refillInterval := config.Window / time.Duration(config.RequestsPerWindow)
	ticker := time.NewTicker(refillInterval)

	rl := &RateLimiter{
		tokens:    tokens,
		ticker:    ticker,
		maxTokens: config.BurstLimit,
		window:    config.Window,
	}

	// Start token refill goroutine
	go rl.refillTokens()

	return rl, nil
}

// Allow checks if a request should be allowed based on rate limit
func (rl *RateLimiter) Allow() bool {
	rl.mu.RLock()
	stopped := rl.stopped
	rl.mu.RUnlock()

	if stopped {
		return false
	}

	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// Wait blocks until a request can be made or context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.RLock()
	stopped := rl.stopped
	rl.mu.RUnlock()

	if stopped {
		return fmt.Errorf("rate limiter stopped")
	}

	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the rate limiter and releases resources
func (rl *RateLimiter) Stop() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.stopped {
		rl.stopped = true
		rl.ticker.Stop()
		close(rl.tokens)
	}
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() RateLimiterStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return RateLimiterStats{
		AvailableTokens: len(rl.tokens),
		MaxTokens:       rl.maxTokens,
		Window:          rl.window,
		Stopped:         rl.stopped,
	}
}

// RateLimiterStats provides statistics about rate limiter state
type RateLimiterStats struct {
	AvailableTokens int
	MaxTokens       int
	Window          time.Duration
	Stopped         bool
}

func (rl *RateLimiter) refillTokens() {
	for range rl.ticker.C {
		rl.mu.RLock()
		stopped := rl.stopped
		rl.mu.RUnlock()

		if stopped {
			return
		}

		// Try to add a token if not at capacity
		select {
		case rl.tokens <- struct{}{}:
			// Token added successfully
		default:
			// Token bucket is full, skip
		}
	}
}

// CircuitBreaker implements circuit breaker pattern for failure handling
type CircuitBreaker struct {
	mu                sync.RWMutex
	state             CircuitState
	failureCount      int
	successCount      int
	lastFailureTime   time.Time
	timeout           time.Duration
	maxFailures       int
	successThreshold  int
	halfOpenRequests  int
	maxHalfOpenReqs   int
}

// CircuitState represents the current state of the circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateHalfOpen
	StateOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	MaxFailures       int           // Maximum failures before opening circuit
	Timeout           time.Duration // Time to wait before attempting to close circuit
	SuccessThreshold  int           // Successful requests needed to close circuit in half-open state
	MaxHalfOpenReqs   int           // Maximum requests allowed in half-open state
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures <= 0 {
		config.MaxFailures = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 3
	}
	if config.MaxHalfOpenReqs <= 0 {
		config.MaxHalfOpenReqs = 5
	}

	return &CircuitBreaker{
		state:            StateClosed,
		timeout:          config.Timeout,
		maxFailures:      config.MaxFailures,
		successThreshold: config.SuccessThreshold,
		maxHalfOpenReqs:  config.MaxHalfOpenReqs,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return fmt.Errorf("circuit breaker is open")
	}

	err := fn()
	cb.recordResult(err == nil)
	return err
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.state = StateHalfOpen
			cb.halfOpenRequests = 0
			return true
		}
		return false
	case StateHalfOpen:
		return cb.halfOpenRequests < cb.maxHalfOpenReqs
	default:
		return false
	}
}

func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.successCount++
		cb.failureCount = 0 // Reset failure count on success

		if cb.state == StateHalfOpen && cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.successCount = 0
		}
	} else {
		cb.failureCount++
		cb.successCount = 0 // Reset success count on failure
		cb.lastFailureTime = time.Now()

		if cb.state == StateClosed && cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
		} else if cb.state == StateHalfOpen {
			cb.state = StateOpen
		}
	}

	if cb.state == StateHalfOpen {
		cb.halfOpenRequests++
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:            cb.state,
		FailureCount:     cb.failureCount,
		SuccessCount:     cb.successCount,
		LastFailureTime:  cb.lastFailureTime,
		HalfOpenRequests: cb.halfOpenRequests,
	}
}

// CircuitBreakerStats provides statistics about circuit breaker state
type CircuitBreakerStats struct {
	State            CircuitState
	FailureCount     int
	SuccessCount     int
	LastFailureTime  time.Time
	HalfOpenRequests int
}

// BackoffStrategy defines different backoff strategies for retries
type BackoffStrategy interface {
	NextDelay(attempt int) time.Duration
	MaxAttempts() int
}

// ExponentialBackoff implements exponential backoff strategy
type ExponentialBackoff struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Attempts    int
}

// NewExponentialBackoff creates a new exponential backoff strategy
func NewExponentialBackoff(baseDelay, maxDelay time.Duration, multiplier float64, maxAttempts int) *ExponentialBackoff {
	if multiplier <= 1.0 {
		multiplier = 2.0
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	return &ExponentialBackoff{
		BaseDelay:  baseDelay,
		MaxDelay:   maxDelay,
		Multiplier: multiplier,
		Attempts:   maxAttempts,
	}
}

// NextDelay calculates the next delay for the given attempt
func (eb *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return eb.BaseDelay
	}

	delay := eb.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * eb.Multiplier)
		if delay > eb.MaxDelay {
			return eb.MaxDelay
		}
	}

	return delay
}

// MaxAttempts returns the maximum number of attempts
func (eb *ExponentialBackoff) MaxAttempts() int {
	return eb.Attempts
}

// LinearBackoff implements linear backoff strategy
type LinearBackoff struct {
	BaseDelay time.Duration
	Increment time.Duration
	MaxDelay  time.Duration
	Attempts  int
}

// NewLinearBackoff creates a new linear backoff strategy
func NewLinearBackoff(baseDelay, increment, maxDelay time.Duration, maxAttempts int) *LinearBackoff {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	return &LinearBackoff{
		BaseDelay: baseDelay,
		Increment: increment,
		MaxDelay:  maxDelay,
		Attempts:  maxAttempts,
	}
}

// NextDelay calculates the next delay for the given attempt
func (lb *LinearBackoff) NextDelay(attempt int) time.Duration {
	delay := lb.BaseDelay + time.Duration(attempt)*lb.Increment
	if delay > lb.MaxDelay {
		return lb.MaxDelay
	}
	return delay
}

// MaxAttempts returns the maximum number of attempts
func (lb *LinearBackoff) MaxAttempts() int {
	return lb.Attempts
}

// RetryWithBackoff executes a function with retry logic and backoff strategy
func RetryWithBackoff(ctx context.Context, fn func() error, strategy BackoffStrategy) error {
	var lastErr error

	for attempt := 0; attempt < strategy.MaxAttempts(); attempt++ {
		if attempt > 0 {
			delay := strategy.NextDelay(attempt - 1)
			
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled after %d attempts: %w", attempt+1, ctx.Err())
		default:
			// Continue with next attempt
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", strategy.MaxAttempts(), lastErr)
}

// RequestTimeoutManager manages timeouts for different types of requests
type RequestTimeoutManager struct {
	defaultTimeout  time.Duration
	operationLimits map[string]time.Duration
	mu              sync.RWMutex
}

// NewRequestTimeoutManager creates a new timeout manager
func NewRequestTimeoutManager(defaultTimeout time.Duration) *RequestTimeoutManager {
	return &RequestTimeoutManager{
		defaultTimeout:  defaultTimeout,
		operationLimits: make(map[string]time.Duration),
	}
}

// SetOperationTimeout sets a specific timeout for an operation type
func (rtm *RequestTimeoutManager) SetOperationTimeout(operation string, timeout time.Duration) {
	rtm.mu.Lock()
	defer rtm.mu.Unlock()
	rtm.operationLimits[operation] = timeout
}

// GetTimeout returns the timeout for a specific operation
func (rtm *RequestTimeoutManager) GetTimeout(operation string) time.Duration {
	rtm.mu.RLock()
	defer rtm.mu.RUnlock()

	if timeout, exists := rtm.operationLimits[operation]; exists {
		return timeout
	}
	return rtm.defaultTimeout
}

// CreateContext creates a context with timeout for the given operation
func (rtm *RequestTimeoutManager) CreateContext(operation string) (context.Context, context.CancelFunc) {
	timeout := rtm.GetTimeout(operation)
	return context.WithTimeout(context.Background(), timeout)
}

// CreateContextWithParent creates a context with timeout using a parent context
func (rtm *RequestTimeoutManager) CreateContextWithParent(parent context.Context, operation string) (context.Context, context.CancelFunc) {
	timeout := rtm.GetTimeout(operation)
	return context.WithTimeout(parent, timeout)
}