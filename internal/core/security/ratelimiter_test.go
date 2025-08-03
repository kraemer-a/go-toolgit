package security

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_BasicFunctionality(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerWindow: 2,
		Window:           time.Second,
		BurstLimit:       2,
	}

	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Stop()

	// First two requests should be allowed (burst limit)
	if !rl.Allow() {
		t.Error("First request should be allowed")
	}
	if !rl.Allow() {
		t.Error("Second request should be allowed")
	}

	// Third request should be denied (exceeds burst limit)
	if rl.Allow() {
		t.Error("Third request should be denied")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerWindow: 4, // 4 requests per second = 1 request per 250ms
		Window:           time.Second,
		BurstLimit:       1, // Only 1 token at a time
	}

	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Stop()

	// Use the initial token
	if !rl.Allow() {
		t.Error("Initial request should be allowed")
	}

	// Should be denied immediately
	if rl.Allow() {
		t.Error("Second immediate request should be denied")
	}

	// Wait for refill (slightly more than 250ms)
	time.Sleep(300 * time.Millisecond)

	// Should be allowed after refill
	if !rl.Allow() {
		t.Error("Request after refill should be allowed")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerWindow: 2,
		Window:           time.Second,
		BurstLimit:       1,
	}

	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Stop()

	// Use the initial token
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := rl.Wait(ctx); err != nil {
		t.Errorf("First wait should succeed: %v", err)
	}

	// Second wait should succeed after refill
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	start := time.Now()
	if err := rl.Wait(ctx2); err != nil {
		t.Errorf("Second wait should succeed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have waited for refill (at least 400ms for 2 req/sec rate)
	if elapsed < 400*time.Millisecond {
		t.Errorf("Expected to wait at least 400ms, waited %v", elapsed)
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerWindow: 1,
		Window:           time.Second,
		BurstLimit:       1,
	}

	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Stop()

	// Use the initial token
	if !rl.Allow() {
		t.Error("First request should be allowed")
	}

	// Create context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait should fail due to context cancellation
	if err := rl.Wait(ctx); err == nil {
		t.Error("Expected context cancellation error")
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerWindow: 2,
		Window:           time.Second,
		BurstLimit:       3,
	}

	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Stop()

	stats := rl.GetStats()
	if stats.MaxTokens != 3 {
		t.Errorf("Expected max tokens 3, got %d", stats.MaxTokens)
	}
	if stats.AvailableTokens != 3 {
		t.Errorf("Expected available tokens 3, got %d", stats.AvailableTokens)
	}

	// Use some tokens
	rl.Allow()
	rl.Allow()

	stats = rl.GetStats()
	if stats.AvailableTokens != 1 {
		t.Errorf("Expected available tokens 1, got %d", stats.AvailableTokens)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerWindow: 10,
		Window:           time.Second,
		BurstLimit:       5,
	}

	rl, err := NewRateLimiter(config)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}
	defer rl.Stop()

	var wg sync.WaitGroup
	allowed := make(chan bool, 20)

	// Launch multiple goroutines
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- rl.Allow()
		}()
	}

	wg.Wait()
	close(allowed)

	// Count allowed requests
	count := 0
	for a := range allowed {
		if a {
			count++
		}
	}

	// Should only allow burst limit requests
	if count != 5 {
		t.Errorf("Expected 5 allowed requests, got %d", count)
	}
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:      3,
		Timeout:          time.Second,
		SuccessThreshold: 2,
	}

	cb := NewCircuitBreaker(config)

	// Circuit should start closed
	if cb.GetState() != StateClosed {
		t.Error("Circuit breaker should start in closed state")
	}

	// Successful executions should work
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("Successful execution should work: %v", err)
	}

	if cb.GetState() != StateClosed {
		t.Error("Circuit should remain closed after success")
	}
}

func TestCircuitBreaker_OpenState(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:      2,
		Timeout:          100 * time.Millisecond,
		SuccessThreshold: 2,
	}

	cb := NewCircuitBreaker(config)

	// Cause failures to open circuit
	cb.Execute(func() error { return fmt.Errorf("error 1") })
	cb.Execute(func() error { return fmt.Errorf("error 2") })

	if cb.GetState() != StateOpen {
		t.Error("Circuit should be open after max failures")
	}

	// Execution should fail immediately
	err := cb.Execute(func() error { return nil })
	if err == nil {
		t.Error("Execution should fail when circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:      2,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 2,
	}

	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.Execute(func() error { return fmt.Errorf("error 1") })
	cb.Execute(func() error { return fmt.Errorf("error 2") })

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Next execution should move to half-open
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("First execution after timeout should succeed: %v", err)
	}

	if cb.GetState() != StateHalfOpen {
		t.Error("Circuit should be in half-open state")
	}

	// One more success should close the circuit
	cb.Execute(func() error { return nil })

	if cb.GetState() != StateClosed {
		t.Error("Circuit should be closed after successful half-open period")
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:      3,
		Timeout:          time.Second,
		SuccessThreshold: 2,
	}

	cb := NewCircuitBreaker(config)

	stats := cb.GetStats()
	if stats.State != StateClosed {
		t.Error("Initial state should be closed")
	}

	// Record some failures
	cb.Execute(func() error { return fmt.Errorf("error") })
	cb.Execute(func() error { return fmt.Errorf("error") })

	stats = cb.GetStats()
	if stats.FailureCount != 2 {
		t.Errorf("Expected failure count 2, got %d", stats.FailureCount)
	}

	// Record success
	cb.Execute(func() error { return nil })

	stats = cb.GetStats()
	if stats.SuccessCount != 1 {
		t.Errorf("Expected success count 1, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 0 {
		t.Errorf("Expected failure count reset to 0, got %d", stats.FailureCount)
	}
}

func TestExponentialBackoff(t *testing.T) {
	backoff := NewExponentialBackoff(100*time.Millisecond, 2*time.Second, 2.0, 5)

	delays := []time.Duration{
		backoff.NextDelay(0), // 100ms
		backoff.NextDelay(1), // 200ms
		backoff.NextDelay(2), // 400ms
		backoff.NextDelay(3), // 800ms
		backoff.NextDelay(4), // 1600ms
		backoff.NextDelay(5), // capped at 2000ms
	}

	expected := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
		2000 * time.Millisecond, // capped
	}

	for i, delay := range delays {
		if delay != expected[i] {
			t.Errorf("Attempt %d: expected %v, got %v", i, expected[i], delay)
		}
	}

	if backoff.MaxAttempts() != 5 {
		t.Errorf("Expected max attempts 5, got %d", backoff.MaxAttempts())
	}
}

func TestLinearBackoff(t *testing.T) {
	backoff := NewLinearBackoff(100*time.Millisecond, 50*time.Millisecond, 500*time.Millisecond, 4)

	delays := []time.Duration{
		backoff.NextDelay(0), // 100ms
		backoff.NextDelay(1), // 150ms
		backoff.NextDelay(2), // 200ms
		backoff.NextDelay(3), // 250ms
		backoff.NextDelay(10), // capped at 500ms
	}

	expected := []time.Duration{
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		250 * time.Millisecond,
		500 * time.Millisecond, // capped
	}

	for i, delay := range delays {
		if delay != expected[i] {
			t.Errorf("Attempt %d: expected %v, got %v", i, expected[i], delay)
		}
	}
}

func TestRetryWithBackoff_Success(t *testing.T) {
	backoff := NewExponentialBackoff(10*time.Millisecond, 100*time.Millisecond, 2.0, 3)
	ctx := context.Background()

	attempts := 0
	err := RetryWithBackoff(ctx, func() error {
		attempts++
		if attempts < 2 {
			return fmt.Errorf("temporary error")
		}
		return nil
	}, backoff)

	if err != nil {
		t.Errorf("Expected success after retries: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_Failure(t *testing.T) {
	backoff := NewLinearBackoff(10*time.Millisecond, 5*time.Millisecond, 50*time.Millisecond, 3)
	ctx := context.Background()

	attempts := 0
	err := RetryWithBackoff(ctx, func() error {
		attempts++
		return fmt.Errorf("persistent error")
	}, backoff)

	if err == nil {
		t.Error("Expected failure after max attempts")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	backoff := NewExponentialBackoff(100*time.Millisecond, time.Second, 2.0, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	err := RetryWithBackoff(ctx, func() error {
		attempts++
		return fmt.Errorf("error")
	}, backoff)

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	// Should have attempted at least once
	if attempts < 1 {
		t.Errorf("Expected at least 1 attempt, got %d", attempts)
	}
}

func TestRequestTimeoutManager(t *testing.T) {
	rtm := NewRequestTimeoutManager(5 * time.Second)

	// Test default timeout
	if rtm.GetTimeout("unknown_operation") != 5*time.Second {
		t.Error("Should return default timeout for unknown operation")
	}

	// Set specific timeout
	rtm.SetOperationTimeout("search", 2*time.Second)
	if rtm.GetTimeout("search") != 2*time.Second {
		t.Error("Should return specific timeout for search operation")
	}

	// Test context creation
	ctx, cancel := rtm.CreateContext("search")
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Context should have deadline")
	}

	expectedDeadline := time.Now().Add(2 * time.Second)
	if deadline.Before(expectedDeadline.Add(-100*time.Millisecond)) ||
		deadline.After(expectedDeadline.Add(100*time.Millisecond)) {
		t.Error("Context deadline should be approximately 2 seconds from now")
	}
}

// Benchmark tests
func BenchmarkRateLimiter_Allow(b *testing.B) {
	config := RateLimitConfig{
		RequestsPerWindow: 1000,
		Window:           time.Second,
		BurstLimit:       100,
	}

	rl, _ := NewRateLimiter(config)
	defer rl.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow()
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	config := CircuitBreakerConfig{
		MaxFailures:      10,
		Timeout:          time.Second,
		SuccessThreshold: 3,
	}

	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() error { return nil })
	}
}

func BenchmarkExponentialBackoff_NextDelay(b *testing.B) {
	backoff := NewExponentialBackoff(10*time.Millisecond, time.Second, 2.0, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff.NextDelay(i % 10)
	}
}