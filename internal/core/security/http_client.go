package security

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// SecureHTTPClient provides a secure HTTP client with rate limiting, circuit breaking, and timeout controls
type SecureHTTPClient struct {
	client          *http.Client
	rateLimiter     *RateLimiter
	circuitBreaker  *CircuitBreaker
	timeoutManager  *RequestTimeoutManager
	validator       *InputValidator
	mu              sync.RWMutex
	requestStats    map[string]*RequestStats
}

// SecureHTTPConfig configures the secure HTTP client
type SecureHTTPConfig struct {
	// HTTP client settings
	Timeout         time.Duration
	MaxIdleConns    int
	IdleConnTimeout time.Duration

	// Rate limiting
	RateLimit RateLimitConfig

	// Circuit breaker
	CircuitBreaker CircuitBreakerConfig

	// Request timeouts for specific operations
	OperationTimeouts map[string]time.Duration

	// Validation settings
	StrictMode bool
}

// RequestStats tracks statistics for requests
type RequestStats struct {
	TotalRequests    int64
	SuccessfulReqs   int64
	FailedRequests   int64
	RateLimitedReqs  int64
	CircuitOpenReqs  int64
	LastRequestTime  time.Time
	AverageLatency   time.Duration
	mu               sync.RWMutex
}

// NewSecureHTTPClient creates a new secure HTTP client
func NewSecureHTTPClient(config SecureHTTPConfig) (*SecureHTTPClient, error) {
	// Validate configuration
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = 10
	}
	if config.IdleConnTimeout <= 0 {
		config.IdleConnTimeout = 90 * time.Second
	}

	// Create HTTP client with security settings
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		IdleConnTimeout:     config.IdleConnTimeout,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 2,
	}

	client := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	// Create rate limiter
	rateLimiter, err := NewRateLimiter(config.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}

	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(config.CircuitBreaker)

	// Create timeout manager
	timeoutManager := NewRequestTimeoutManager(config.Timeout)
	for operation, timeout := range config.OperationTimeouts {
		timeoutManager.SetOperationTimeout(operation, timeout)
	}

	// Create validator
	validator := NewInputValidator(config.StrictMode)

	return &SecureHTTPClient{
		client:         client,
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
		timeoutManager: timeoutManager,
		validator:      validator,
		requestStats:   make(map[string]*RequestStats),
	}, nil
}

// Get performs a secure GET request
func (sc *SecureHTTPClient) Get(operation, urlStr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return sc.Do(operation, req)
}

// Post performs a secure POST request
func (sc *SecureHTTPClient) Post(operation, urlStr, contentType string, body interface{}) (*http.Response, error) {
	// Implementation would depend on body type (io.Reader, string, etc.)
	req, err := http.NewRequest(http.MethodPost, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	return sc.Do(operation, req)
}

// Do performs a secure HTTP request with all security controls
func (sc *SecureHTTPClient) Do(operation string, req *http.Request) (*http.Response, error) {
	// Validate request URL
	if err := sc.validator.ValidateURL("url", req.URL.String()); err != nil {
		sc.recordStats(operation, false, true, false, false, 0)
		return nil, fmt.Errorf("invalid request URL: %w", err)
	}

	// Check rate limit
	if !sc.rateLimiter.Allow() {
		sc.recordStats(operation, false, false, true, false, 0)
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Create context with timeout
	ctx, cancel := sc.timeoutManager.CreateContextWithParent(req.Context(), operation)
	defer cancel()

	// Update request with timeout context
	req = req.WithContext(ctx)

	// Execute with circuit breaker
	var resp *http.Response
	var err error
	start := time.Now()

	cbErr := sc.circuitBreaker.Execute(func() error {
		resp, err = sc.client.Do(req)
		if err != nil {
			return err
		}
		// Treat HTTP error status codes as failures for circuit breaker
		if resp != nil && resp.StatusCode >= 500 {
			return fmt.Errorf("HTTP %d error", resp.StatusCode)
		}
		return nil
	})

	latency := time.Since(start)

	if cbErr != nil {
		if cbErr.Error() == "circuit breaker is open" {
			sc.recordStats(operation, false, false, false, true, latency)
			return nil, fmt.Errorf("circuit breaker is open, request rejected")
		}
		sc.recordStats(operation, false, false, false, false, latency)
		return nil, fmt.Errorf("request failed: %w", cbErr)
	}

	// Check response status
	success := resp != nil && resp.StatusCode < 400
	sc.recordStats(operation, success, false, false, false, latency)

	return resp, nil
}

// DoWithRetry performs a request with retry logic (bypasses circuit breaker for internal retries)
func (sc *SecureHTTPClient) DoWithRetry(operation string, req *http.Request, strategy BackoffStrategy) (*http.Response, error) {
	var resp *http.Response
	
	// Use circuit breaker wrapper for the entire retry operation
	cbErr := sc.circuitBreaker.Execute(func() error {
		return RetryWithBackoff(req.Context(), func() error {
			var reqErr error
			resp, reqErr = sc.doWithoutCircuitBreaker(operation, req)
			
			// Network/connection errors should be retried
			if reqErr != nil {
				return reqErr
			}
			
			// HTTP errors: 4xx shouldn't be retried, 5xx should be retried
			if resp != nil {
				if resp.StatusCode >= 500 {
					return fmt.Errorf("HTTP %d error", resp.StatusCode)
				}
				if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					// Client errors shouldn't be retried
					return nil // Success from retry perspective 
				}
			}
			
			return nil // Success
		}, strategy)
	})

	if cbErr != nil {
		if cbErr.Error() == "circuit breaker is open" {
			return nil, fmt.Errorf("circuit breaker is open, request rejected")
		}
		return nil, cbErr
	}

	return resp, nil
}

// doWithoutCircuitBreaker performs HTTP request without circuit breaker (for internal retries)
func (sc *SecureHTTPClient) doWithoutCircuitBreaker(operation string, req *http.Request) (*http.Response, error) {
	// Validate request URL
	if err := sc.validator.ValidateURL("url", req.URL.String()); err != nil {
		sc.recordStats(operation, false, true, false, false, 0)
		return nil, fmt.Errorf("invalid request URL: %w", err)
	}

	// Check rate limit
	if !sc.rateLimiter.Allow() {
		sc.recordStats(operation, false, false, true, false, 0)
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Create context with timeout
	ctx, cancel := sc.timeoutManager.CreateContextWithParent(req.Context(), operation)
	defer cancel()

	// Update request with timeout context
	req = req.WithContext(ctx)

	// Execute request directly
	start := time.Now()
	resp, err := sc.client.Do(req)
	latency := time.Since(start)

	// Check response status
	success := resp != nil && resp.StatusCode < 400
	sc.recordStats(operation, success, false, false, false, latency)

	return resp, err
}

// ValidateAndSanitizeHeaders validates and sanitizes HTTP headers
func (sc *SecureHTTPClient) ValidateAndSanitizeHeaders(headers map[string]string) (map[string]string, error) {
	sanitized := make(map[string]string)

	for key, value := range headers {
		// Validate header name
		if err := sc.validator.ValidateString("header_name", key, 100); err != nil {
			return nil, fmt.Errorf("invalid header name '%s': %w", key, err)
		}

		// Validate header value
		if err := sc.validator.ValidateString("header_value", value, 1000); err != nil {
			return nil, fmt.Errorf("invalid header value for '%s': %w", key, err)
		}

		// Sanitize value
		sanitizedValue := sc.validator.SanitizeString(value)
		sanitized[key] = sanitizedValue
	}

	return sanitized, nil
}

// ValidateAndSanitizeQueryParams validates and sanitizes URL query parameters
func (sc *SecureHTTPClient) ValidateAndSanitizeQueryParams(params map[string]string) (url.Values, error) {
	values := url.Values{}

	for key, value := range params {
		// Validate parameter name
		if err := sc.validator.ValidateString("param_name", key, 100); err != nil {
			return nil, fmt.Errorf("invalid parameter name '%s': %w", key, err)
		}

		// Validate parameter value
		if err := sc.validator.ValidateString("param_value", value, 2000); err != nil {
			return nil, fmt.Errorf("invalid parameter value for '%s': %w", key, err)
		}

		// Sanitize value
		sanitizedValue := sc.validator.SanitizeString(value)
		values.Set(key, sanitizedValue)
	}

	return values, nil
}

// GetStats returns statistics for all operations
func (sc *SecureHTTPClient) GetStats() map[string]RequestStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	stats := make(map[string]RequestStats)
	for operation, stat := range sc.requestStats {
		stat.mu.RLock()
		stats[operation] = *stat
		stat.mu.RUnlock()
	}

	return stats
}

// GetOperationStats returns statistics for a specific operation
func (sc *SecureHTTPClient) GetOperationStats(operation string) (RequestStats, bool) {
	sc.mu.RLock()
	stat, exists := sc.requestStats[operation]
	sc.mu.RUnlock()

	if !exists {
		return RequestStats{}, false
	}

	stat.mu.RLock()
	defer stat.mu.RUnlock()
	return *stat, true
}

// GetSystemStats returns overall system statistics
func (sc *SecureHTTPClient) GetSystemStats() SystemStats {
	return SystemStats{
		RateLimiter:    sc.rateLimiter.GetStats(),
		CircuitBreaker: sc.circuitBreaker.GetStats(),
		Timestamp:      time.Now(),
	}
}

// SystemStats provides overall system statistics
type SystemStats struct {
	RateLimiter    RateLimiterStats
	CircuitBreaker CircuitBreakerStats
	Timestamp      time.Time
}

// Close releases resources used by the secure HTTP client
func (sc *SecureHTTPClient) Close() error {
	sc.rateLimiter.Stop()
	
	// Close idle connections
	if transport, ok := sc.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	
	return nil
}

// recordStats records request statistics
func (sc *SecureHTTPClient) recordStats(operation string, success, validationFailed, rateLimited, circuitOpen bool, latency time.Duration) {
	sc.mu.Lock()
	stat, exists := sc.requestStats[operation]
	if !exists {
		stat = &RequestStats{}
		sc.requestStats[operation] = stat
	}
	sc.mu.Unlock()

	stat.mu.Lock()
	defer stat.mu.Unlock()

	stat.TotalRequests++
	stat.LastRequestTime = time.Now()

	if success {
		stat.SuccessfulReqs++
	} else if !validationFailed && !rateLimited && !circuitOpen {
		stat.FailedRequests++
	}

	if rateLimited {
		stat.RateLimitedReqs++
	}

	if circuitOpen {
		stat.CircuitOpenReqs++
	}

	// Update average latency
	if stat.TotalRequests == 1 {
		stat.AverageLatency = latency
	} else {
		// Moving average
		stat.AverageLatency = time.Duration(
			(int64(stat.AverageLatency)*stat.TotalRequests + int64(latency)) / (stat.TotalRequests + 1),
		)
	}
}

// SecureRequestBuilder helps build secure HTTP requests
type SecureRequestBuilder struct {
	client   *SecureHTTPClient
	method   string
	url      string
	headers  map[string]string
	params   map[string]string
	body     interface{}
}

// NewSecureRequestBuilder creates a new request builder
func (sc *SecureHTTPClient) NewRequestBuilder(method, url string) *SecureRequestBuilder {
	return &SecureRequestBuilder{
		client:  sc,
		method:  method,
		url:     url,
		headers: make(map[string]string),
		params:  make(map[string]string),
	}
}

// SetHeader sets a header for the request
func (srb *SecureRequestBuilder) SetHeader(key, value string) *SecureRequestBuilder {
	srb.headers[key] = value
	return srb
}

// SetParam sets a query parameter for the request
func (srb *SecureRequestBuilder) SetParam(key, value string) *SecureRequestBuilder {
	srb.params[key] = value
	return srb
}

// SetBody sets the request body
func (srb *SecureRequestBuilder) SetBody(body interface{}) *SecureRequestBuilder {
	srb.body = body
	return srb
}

// Build builds and validates the HTTP request
func (srb *SecureRequestBuilder) Build() (*http.Request, error) {
	// Validate and sanitize URL
	if err := srb.client.validator.ValidateURL("url", srb.url); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Parse URL
	parsedURL, err := url.Parse(srb.url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Validate and sanitize query parameters
	if len(srb.params) > 0 {
		params, err := srb.client.ValidateAndSanitizeQueryParams(srb.params)
		if err != nil {
			return nil, fmt.Errorf("invalid query parameters: %w", err)
		}
		parsedURL.RawQuery = params.Encode()
	}

	// Create request
	req, err := http.NewRequest(srb.method, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Validate and sanitize headers
	if len(srb.headers) > 0 {
		headers, err := srb.client.ValidateAndSanitizeHeaders(srb.headers)
		if err != nil {
			return nil, fmt.Errorf("invalid headers: %w", err)
		}

		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	return req, nil
}

// Execute builds and executes the request
func (srb *SecureRequestBuilder) Execute(operation string) (*http.Response, error) {
	req, err := srb.Build()
	if err != nil {
		return nil, err
	}

	return srb.client.Do(operation, req)
}

// ExecuteWithRetry builds and executes the request with retry logic
func (srb *SecureRequestBuilder) ExecuteWithRetry(operation string, strategy BackoffStrategy) (*http.Response, error) {
	req, err := srb.Build()
	if err != nil {
		return nil, err
	}

	return srb.client.DoWithRetry(operation, req, strategy)
}