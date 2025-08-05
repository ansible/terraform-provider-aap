package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/stretchr/testify/assert"
)

func TestRetryOperation(t *testing.T) {
	testInitialDelay := int64(1) // 1 second for testing
	testRetryDelay := int64(1)   // 1 second for testing
	successCodes := []int{http.StatusOK}
	retryableCodes := []int{http.StatusConflict}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"current_version": "/api/v2/"}`)) //nolint:errcheck
		}
	}))
	defer server.Close()

	// Create a test client (not used in these specific tests but needed for setup)
	username := "testuser"
	password := "testpass"
	_, diags := NewClient(server.URL, &username, &password, true, 30)
	if diags.HasError() {
		t.Fatalf("Failed to create test client: %v", diags)
	}

	t.Run("operation succeeds on the first attempt", func(t *testing.T) {
		ctx := context.Background()
		expectedBody := []byte(`{"message": "operation successful"}`)
		operationName := "testSuccessOperation"
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			return expectedBody, nil, http.StatusOK
		}

		// --- Act ---
		retryConfig := CreateRetryConfig(ctx, operationName, mockOperation, successCodes, retryableCodes, 120, testInitialDelay, testRetryDelay)
		result, err := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.NoError(t, err, "RetryWithConfig should not return an error on a successful operation")
		assert.Equal(t, expectedBody, result, "The result body should match the one returned by the successful operation")
	})

	t.Run("operation succeeds after a conflict", func(t *testing.T) {
		// --- Setup ---
		ctx := context.Background()
		expectedBody := []byte(`{"message": "operation eventually successful"}`)
		operationName := "testConflictThenSuccessOperation"
		callCount := 0
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			callCount++
			if callCount == 1 {
				return nil, nil, http.StatusConflict
			}
			return expectedBody, nil, http.StatusOK
		}

		// --- Act ---
		startTime := time.Now()
		retryConfig := CreateRetryConfig(ctx, operationName, mockOperation, successCodes, retryableCodes, 120, testInitialDelay, testRetryDelay)
		result, err := RetryWithConfig(retryConfig)
		elapsedTime := time.Since(startTime)

		// --- Assert ---
		assert.NoError(t, err, "RetryOperation should not return an error on eventual success")
		assert.Equal(t, expectedBody, result, "The result body should match the one from the successful call")
		assert.Equal(t, 2, callCount, "The mock operation should have been called twice")
		expectedMinDuration := time.Duration(testInitialDelay+testRetryDelay) * time.Second
		assert.GreaterOrEqual(t, elapsedTime, expectedMinDuration, "The elapsed time should be at least the initial delay plus the retry delay")
	})

	t.Run("operation_fails_immediately_on_non_retryable_error", func(t *testing.T) {
		// --- Setup ---
		ctx := context.Background()
		operationName := "testNonRetryableError"
		callCount := 0
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			callCount++
			return nil, nil, http.StatusBadRequest
		}

		// --- Act ---
		retryConfig := CreateRetryConfig(ctx, operationName, mockOperation, successCodes, retryableCodes, 120, testInitialDelay, testRetryDelay)
		_, err := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.Error(t, err, "RetryOperation should return an error for a non-retryable status")
		if err != nil {
			assert.Contains(t, err.Error(), "non-retryable", "The error message should indicate a non-retryable error")
		}
		assert.Equal(t, 1, callCount, "The mock operation should have been called only once")
	})
}
