package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecureHTTPClient_Creation(t *testing.T) {
	config := SecureHTTPConfig{
		Timeout:         30 * time.Second,
		MaxIdleConns:    10,
		IdleConnTimeout: 90 * time.Second,
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      3,
			Timeout:          30 * time.Second,
			SuccessThreshold: 2,
		},
		OperationTimeouts: map[string]time.Duration{
			"search": 5 * time.Second,
			"upload": 30 * time.Second,
		},
		StrictMode: true,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	if client.client.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.client.Timeout)
	}
}

func TestSecureHTTPClient_RateLimiting(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 2,
			Window:           time.Second,
			BurstLimit:       2,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      10,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// First two requests should succeed
	_, err = client.Get("test", server.URL)
	if err != nil {
		t.Errorf("First request should succeed: %v", err)
	}

	_, err = client.Get("test", server.URL)
	if err != nil {
		t.Errorf("Second request should succeed: %v", err)
	}

	// Third request should be rate limited
	_, err = client.Get("test", server.URL)
	if err == nil {
		t.Error("Third request should be rate limited")
	}
	if err != nil && err.Error() != "rate limit exceeded" {
		t.Errorf("Expected rate limit error, got: %v", err)
	}
}

func TestSecureHTTPClient_CircuitBreaker(t *testing.T) {
	// Create test server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 100,
			Window:           time.Second,
			BurstLimit:       10,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      2,
			Timeout:          100 * time.Millisecond,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// First two requests should reach the server (but get 500 errors)
	_, err = client.Get("test", server.URL)
	if err == nil {
		t.Error("Expected HTTP error for first request")
	}

	_, err = client.Get("test", server.URL)
	if err == nil {
		t.Error("Expected HTTP error for second request")
	}

	// Third request should be rejected by circuit breaker
	_, err = client.Get("test", server.URL)
	if err == nil {
		t.Error("Expected circuit breaker to reject third request")
	}
	if err != nil && err.Error() != "circuit breaker is open, request rejected" {
		t.Errorf("Expected circuit breaker error, got: %v", err)
	}
}

func TestSecureHTTPClient_URLValidation(t *testing.T) {
	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: true,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Test malicious URLs
	maliciousURLs := []string{
		"http://evil.com<script>alert('xss')</script>",
		"https://example.com; rm -rf /",
		"file:///etc/passwd",
		"ftp://malicious.com",
	}

	for _, url := range maliciousURLs {
		_, err = client.Get("test", url)
		if err == nil {
			t.Errorf("Expected validation error for malicious URL: %s", url)
		}
	}
}

func TestSecureHTTPClient_RequestBuilder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("X-Test-Header") != "test-value" {
			t.Errorf("Expected header X-Test-Header=test-value")
		}

		// Check query parameters
		if r.URL.Query().Get("param1") != "value1" {
			t.Errorf("Expected query parameter param1=value1")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Use request builder
	resp, err := client.NewRequestBuilder("GET", server.URL).
		SetHeader("X-Test-Header", "test-value").
		SetParam("param1", "value1").
		Execute("test")

	if err != nil {
		t.Errorf("Request should succeed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestSecureHTTPClient_HeaderValidation(t *testing.T) {
	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: true,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Test malicious headers
	maliciousHeaders := map[string]string{
		"X-Command":      "rm -rf /",
		"X-Script":       "<script>alert('xss')</script>",
		"X-Injection":    "'; DROP TABLE users; --",
		"X-Control":      "test\x00control",
	}

	_, err = client.ValidateAndSanitizeHeaders(maliciousHeaders)
	if err == nil {
		t.Error("Expected validation error for malicious headers")
	}
}

func TestSecureHTTPClient_QueryParamValidation(t *testing.T) {
	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: true,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Test malicious query parameters
	maliciousParams := map[string]string{
		"command":   "rm -rf /",
		"script":    "<script>alert('xss')</script>",
		"injection": "'; DROP TABLE users; --",
	}

	_, err = client.ValidateAndSanitizeQueryParams(maliciousParams)
	if err == nil {
		t.Error("Expected validation error for malicious query parameters")
	}
}

func TestSecureHTTPClient_Statistics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Make some requests
	client.Get("test", server.URL)
	client.Get("test", server.URL)

	// Check statistics
	stats := client.GetStats()
	if testStats, exists := stats["test"]; exists {
		if testStats.TotalRequests != 2 {
			t.Errorf("Expected 2 total requests, got %d", testStats.TotalRequests)
		}
		if testStats.SuccessfulReqs != 2 {
			t.Errorf("Expected 2 successful requests, got %d", testStats.SuccessfulReqs)
		}
	} else {
		t.Error("Expected statistics for 'test' operation")
	}

	// Check system statistics
	systemStats := client.GetSystemStats()
	if systemStats.RateLimiter.MaxTokens != 5 {
		t.Errorf("Expected max tokens 5, got %d", systemStats.RateLimiter.MaxTokens)
	}
}

func TestSecureHTTPClient_Timeouts(t *testing.T) {
	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		Timeout: 100 * time.Millisecond, // Shorter than server delay
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Request should timeout
	_, err = client.Get("test", server.URL)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestSecureHTTPClient_WithRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 100,
			Window:           time.Second,
			BurstLimit:       10,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      10, // High threshold to allow retries
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Create request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Use retry with exponential backoff
	backoff := NewExponentialBackoff(10*time.Millisecond, 100*time.Millisecond, 2.0, 5)
	
	resp, err := client.DoWithRetry("test", req, backoff)
	if err != nil {
		t.Errorf("Request with retry should eventually succeed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestSecureHTTPClient_RequestBuilderValidation(t *testing.T) {
	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 10,
			Window:           time.Second,
			BurstLimit:       5,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      5,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: true,
	}

	client, err := NewSecureHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure HTTP client: %v", err)
	}
	defer client.Close()

	// Test malicious URL in builder
	_, err = client.NewRequestBuilder("GET", "http://evil.com; rm -rf /").
		Build()

	if err == nil {
		t.Error("Expected validation error for malicious URL in builder")
	}

	// Test malicious headers in builder
	_, err = client.NewRequestBuilder("GET", "https://api.github.com").
		SetHeader("X-Command", "rm -rf /").
		Build()

	if err == nil {
		t.Error("Expected validation error for malicious header in builder")
	}

	// Test malicious query parameters in builder
	_, err = client.NewRequestBuilder("GET", "https://api.github.com").
		SetParam("command", "; rm -rf /").
		Build()

	if err == nil {
		t.Error("Expected validation error for malicious query parameter in builder")
	}
}

// Benchmark tests
func BenchmarkSecureHTTPClient_Get(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 1000,
			Window:           time.Second,
			BurstLimit:       100,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      100,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, _ := NewSecureHTTPClient(config)
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Get("benchmark", server.URL)
	}
}

func BenchmarkSecureHTTPClient_RequestBuilder(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	config := SecureHTTPConfig{
		RateLimit: RateLimitConfig{
			RequestsPerWindow: 1000,
			Window:           time.Second,
			BurstLimit:       100,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures:      100,
			Timeout:          time.Second,
			SuccessThreshold: 2,
		},
		StrictMode: false,
	}

	client, _ := NewSecureHTTPClient(config)
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.NewRequestBuilder("GET", server.URL).
			SetHeader("X-Test", "value").
			SetParam("param", "value").
			Execute("benchmark")
	}
}