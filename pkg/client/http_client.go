package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DoWithRetry executes HTTP request with exponential backoff
func DoWithRetry(ctx context.Context, req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Attempt the request with retries
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled before each retry
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
		default:
		}

		// Clone the request for each retry (required because request body can only be read once)
		reqClone := req.Clone(ctx)

		// Execute the HTTP request
		resp, lastErr = client.Do(reqClone)

		// If successful, return immediately
		if lastErr == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// If this was the last retry, break
		if attempt == maxRetries {
			break
		}

		// Calculate exponential backoff delay: 2^attempt * 100ms
		// attempt 0: 100ms, attempt 1: 200ms, attempt 2: 400ms, attempt 3: 800ms
		backoffDelay := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond

		// Cap maximum backoff at 5 seconds
		if backoffDelay > 5*time.Second {
			backoffDelay = 5 * time.Second
		}

		// Log retry attempt (in production, use proper logger)
		if lastErr != nil {
			fmt.Printf("Request failed (attempt %d/%d): %v. Retrying in %v...\n",
				attempt+1, maxRetries+1, lastErr, backoffDelay)
		} else if resp != nil {
			fmt.Printf("Request returned status %d (attempt %d/%d). Retrying in %v...\n",
				resp.StatusCode, attempt+1, maxRetries+1, backoffDelay)
			resp.Body.Close() // Close the response body before retrying
		}

		// Wait for backoff period or context cancellation
		select {
		case <-time.After(backoffDelay):
			// Continue to next retry
		case <-ctx.Done():
			return nil, fmt.Errorf("request cancelled during backoff: %w", ctx.Err())
		}
	}

	// All retries exhausted, return the last error
	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
	}

	// If we have a response but it's still an error status, return it
	if resp != nil && resp.StatusCode >= 500 {
		return resp, fmt.Errorf("request failed after %d retries with status %d", maxRetries, resp.StatusCode)
	}

	return resp, lastErr
}

// ValidateResponse checks HTTP status codes
func ValidateResponse(resp *http.Response) error {
	// Check if response is nil
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	// Status codes 2xx and 3xx are considered successful
	if resp.StatusCode < 400 {
		return nil
	}

	// Read the response body for error details
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// If we can't read the body, return a generic error
		return fmt.Errorf("HTTP %d: %s (failed to read response body: %v)",
			resp.StatusCode, resp.Status, err)
	}

	// Close the body (caller should also close, but this ensures it)
	resp.Body.Close()

	// Convert body to string for error message
	bodyString := string(bodyBytes)

	// Limit body length in error message to avoid huge error messages
	if len(bodyString) > 500 {
		bodyString = bodyString[:500] + "... (truncated)"
	}

	// Return detailed error with status code and body
	return fmt.Errorf("HTTP %d: %s - Response body: %s",
		resp.StatusCode, resp.Status, bodyString)
}

// Helper function to perform a request with both retry and validation
func DoWithRetryAndValidation(ctx context.Context, req *http.Request, maxRetries int) (*http.Response, error) {
	// Execute request with retries
	resp, err := DoWithRetry(ctx, req, maxRetries)
	if err != nil {
		return nil, err
	}

	// Validate the response status code
	if err := ValidateResponse(resp); err != nil {
		return resp, err
	}

	return resp, nil
}