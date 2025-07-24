package provider

import (
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// TestCreateRetryStateChangeConf_ImmediateSuccess tests immediate success scenario
func TestCreateRetryStateChangeConf_ImmediateSuccess(t *testing.T) {
	operation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("success"), diag.Diagnostics{}, http.StatusOK
	}

	stateConf := CreateRetryStateChangeConf(
		operation,
		5*time.Second,
		[]int{http.StatusOK},
		"test operation",
	)

	result, state, err := stateConf.Refresh()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if state != retryStateSuccess {
		t.Errorf("Expected state 'success', got: %s", state)
	}
	if string(result.([]byte)) != "success" {
		t.Errorf("Expected result 'success', got: %s", string(result.([]byte)))
	}
}

// TestCreateRetryStateChangeConf_RetryThenSuccess tests retry then success scenario
func TestCreateRetryStateChangeConf_RetryThenSuccess(t *testing.T) {
	callCount := 0
	operation := func() ([]byte, diag.Diagnostics, int) {
		callCount++
		if callCount == 1 {
			return []byte("conflict"), diag.Diagnostics{}, http.StatusConflict
		}
		return []byte("success"), diag.Diagnostics{}, http.StatusOK
	}

	stateConf := CreateRetryStateChangeConf(
		operation,
		5*time.Second,
		[]int{http.StatusOK},
		"test operation",
	)

	// First call should return retry state
	result, state, err := stateConf.Refresh()
	if err != nil {
		t.Errorf("First call should not error, got: %v", err)
	}
	if state != retryStateRetrying {
		t.Errorf("First call should return 'retrying', got: %s", state)
	}
	if result != nil {
		t.Errorf("First call should return nil result, got: %v", result)
	}

	// Second call should succeed
	result, state, err = stateConf.Refresh()
	if err != nil {
		t.Errorf("Second call should not error, got: %v", err)
	}
	if state != retryStateSuccess {
		t.Errorf("Second call should return 'success', got: %s", state)
	}
	if string(result.([]byte)) != "success" {
		t.Errorf("Expected result 'success', got: %s", string(result.([]byte)))
	}
}

// TestCreateRetryStateChangeConf_NonRetryableError tests non-retryable error scenario
func TestCreateRetryStateChangeConf_NonRetryableError(t *testing.T) {
	operation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("bad request"), diag.Diagnostics{}, http.StatusBadRequest
	}

	stateConf := CreateRetryStateChangeConf(
		operation,
		5*time.Second,
		[]int{http.StatusOK},
		"test operation",
	)

	result, state, err := stateConf.Refresh()
	if err == nil {
		t.Error("Expected error for non-retryable status code")
	}
	if !contains(err.Error(), "non-retryable HTTP status 400") {
		t.Errorf("Expected error message about status 400, got: %s", err.Error())
	}
	if state != "" {
		t.Errorf("Expected empty state on error, got: %s", state)
	}
	if result != nil {
		t.Errorf("Expected nil result on error, got: %v", result)
	}
}

// TestCreateRetryStateChangeConf_ConflictRetryable tests HTTP 409 is retryable
func TestCreateRetryStateChangeConf_ConflictRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusConflict)
}

// TestCreateRetryStateChangeConf_TimeoutRetryable tests HTTP 408 is retryable
func TestCreateRetryStateChangeConf_TimeoutRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusRequestTimeout)
}

// TestCreateRetryStateChangeConf_TooManyRequestsRetryable tests HTTP 429 is retryable
func TestCreateRetryStateChangeConf_TooManyRequestsRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusTooManyRequests)
}

// TestCreateRetryStateChangeConf_InternalServerErrorRetryable tests HTTP 500 is retryable
func TestCreateRetryStateChangeConf_InternalServerErrorRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusInternalServerError)
}

// TestCreateRetryStateChangeConf_BadGatewayRetryable tests HTTP 502 is retryable
func TestCreateRetryStateChangeConf_BadGatewayRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusBadGateway)
}

// TestCreateRetryStateChangeConf_ServiceUnavailableRetryable tests HTTP 503 is retryable
func TestCreateRetryStateChangeConf_ServiceUnavailableRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusServiceUnavailable)
}

// TestCreateRetryStateChangeConf_GatewayTimeoutRetryable tests HTTP 504 is retryable
func TestCreateRetryStateChangeConf_GatewayTimeoutRetryable(t *testing.T) {
	testRetryableStatusCode(t, http.StatusGatewayTimeout)
}

// testRetryableStatusCode is a helper function to test retryable status codes
func testRetryableStatusCode(t *testing.T, statusCode int) {
	operation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("retry"), diag.Diagnostics{}, statusCode
	}

	stateConf := CreateRetryStateChangeConf(
		operation,
		5*time.Second,
		[]int{http.StatusOK},
		"test operation",
	)

	result, state, err := stateConf.Refresh()
	if err != nil {
		t.Errorf("Status code %d should be retryable, got error: %v", statusCode, err)
	}
	if state != retryStateRetrying {
		t.Errorf("Status code %d should return 'retrying', got: %s", statusCode, state)
	}
	if result != nil {
		t.Errorf("Status code %d should return nil result, got: %v", statusCode, result)
	}
}

// TestCreateRetryStateChangeConfConfiguration tests timeout configuration
func TestCreateRetryStateChangeConfConfiguration(t *testing.T) {
	operation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("success"), diag.Diagnostics{}, http.StatusOK
	}

	stateConf := CreateRetryStateChangeConf(
		operation,
		30*time.Second,
		[]int{http.StatusOK},
		"test operation",
	)

	// Verify configuration
	if stateConf.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", stateConf.Timeout)
	}
	if stateConf.MinTimeout != 5*time.Second {
		t.Errorf("Expected MinTimeout 5s (no jitter for short timeout), got %v", stateConf.MinTimeout)
	}
	if stateConf.Delay != 2*time.Second {
		t.Errorf("Expected Delay 2s, got %v", stateConf.Delay)
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
