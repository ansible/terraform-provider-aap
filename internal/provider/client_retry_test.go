package provider

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// TestCreateRetryStateChangeConf tests essential retry scenarios
func TestCreateRetryStateChangeConf(t *testing.T) {
	// Test immediate success
	t.Run("immediate_success", func(t *testing.T) {
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
	})

	// Test retry then success
	t.Run("retry_409_then_success", func(t *testing.T) {
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
	})

	// Test non-retryable error
	t.Run("non_retryable_400", func(t *testing.T) {
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
	})

	// Test all retryable status codes
	retryableCodes := []int{
		http.StatusConflict, http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout,
	}

	for _, code := range retryableCodes {
		t.Run(fmt.Sprintf("retryable_%d", code), func(t *testing.T) {
			operation := func() ([]byte, diag.Diagnostics, int) {
				return []byte("retry"), diag.Diagnostics{}, code
			}

			stateConf := CreateRetryStateChangeConf(
				operation,
				5*time.Second,
				[]int{http.StatusOK},
				"test operation",
			)

			result, state, err := stateConf.Refresh()
			if err != nil {
				t.Errorf("Status code %d should be retryable, got error: %v", code, err)
			}
			if state != retryStateRetrying {
				t.Errorf("Status code %d should return 'retrying', got: %s", code, state)
			}
			if result != nil {
				t.Errorf("Status code %d should return nil result, got: %v", code, result)
			}
		})
	}
}

// Test timeout configuration
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
	if stateConf.MinTimeout != 2*time.Second {
		t.Errorf("Expected MinTimeout 2s (no jitter for short timeout), got %v", stateConf.MinTimeout)
	}
	if stateConf.Delay != 1*time.Second {
		t.Errorf("Expected Delay 1s, got %v", stateConf.Delay)
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
