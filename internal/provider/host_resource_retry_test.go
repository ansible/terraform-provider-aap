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

func TestCalculateTimeout(t *testing.T) {
	// Define the test table
	testTable := []struct {
		name            string
		setupCtx        func() (context.Context, context.CancelFunc)
		expectedTimeout int
	}{
		{
			name: "Context with a standard deadline",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 20*time.Minute)
			},
			expectedTimeout: 19 * 60, // 19 minutes in seconds
		},
		{
			name: "Context with no deadline",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			expectedTimeout: maxTimeoutSeconds,
		},
		{
			name: "Context with a short deadline, clamps to minimum",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 3*time.Second)
			},
			expectedTimeout: minTimeoutSeconds,
		},
		{
			name: "Context with an expired deadline, clamps to minimum",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-10*time.Second))
			},
			expectedTimeout: minTimeoutSeconds,
		},
		{
			name: "Context deadline resulting in exactly minimum timeout",
			setupCtx: func() (context.Context, context.CancelFunc) {
				// With 60s buffer, this will clamp to minimum
				return context.WithTimeout(context.Background(), 6250*time.Millisecond)
			},
			expectedTimeout: minTimeoutSeconds,
		},
		{
			name: "Context with long deadline properly subtracts buffer",
			setupCtx: func() (context.Context, context.CancelFunc) {
				// 90s - 60s buffer = 30s
				return context.WithTimeout(context.Background(), 90*time.Second)
			},
			expectedTimeout: 30,
		},
	}

	// Iterate over the test table
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the context for the test case
			ctx, cancel := tc.setupCtx()
			defer cancel()

			// Calculate the timeout
			timeout := CalculateTimeout(ctx)

			// We round the result to the nearest second for reliable comparison,
			// as the exact remaining time can have minor variations.
			if timeout != tc.expectedTimeout {
				t.Errorf("Expected timeout %v, but got %v", tc.expectedTimeout, timeout)
			}
		})
	}
}

func TestRetryOperation(t *testing.T) {
	testInitialDelay := 10 * time.Millisecond
	testRetryDelay := 5 * time.Millisecond
	successCodes := []int{http.StatusOK}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == apiEndpoint {
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
		expectedBody := []byte(`{"message": "operation successful"}`)
		operationName := "testSuccessOperation"
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			return expectedBody, nil, http.StatusOK
		}

		// --- Act ---
		retryConfig := CreateRetryConfig(operationName, mockOperation, successCodes, 120, testInitialDelay, testRetryDelay)
		result, err := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.NoError(t, err, "RetryWithConfig should not return an error on a successful operation")
		assert.Equal(t, expectedBody, result, "The result body should match the one returned by the successful operation")
	})

	t.Run("operation succeeds after a conflict", func(t *testing.T) {
		// --- Setup ---
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
		retryConfig := CreateRetryConfig(operationName, mockOperation, successCodes, 120, testInitialDelay, testRetryDelay)
		result, err := RetryWithConfig(retryConfig)
		elapsedTime := time.Since(startTime)

		// --- Assert ---
		assert.NoError(t, err, "RetryOperation should not return an error on eventual success")
		assert.Equal(t, expectedBody, result, "The result body should match the one from the successful call")
		assert.Equal(t, 2, callCount, "The mock operation should have been called twice")
		expectedMinDuration := testInitialDelay + testRetryDelay
		assert.GreaterOrEqual(t, elapsedTime, expectedMinDuration, "The elapsed time should be at least the initial delay plus the retry delay")
	})
	t.Run("operation_times_out_if_it_never_succeeds", func(t *testing.T) {
		// --- Setup ---
		operationName := "testConflictThatAlwaysFails"
		callCount := 0
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			callCount++
			return nil, nil, http.StatusConflict
		}

		// --- Act ---
		retryConfig := CreateRetryConfig(operationName, mockOperation, successCodes, 1, testInitialDelay, testRetryDelay)
		_, err := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.Error(t, err, "RetryOperation should return an error when it times out")
		if err != nil {
			assert.Contains(t, err.Error(), "timeout", "The error message should indicate a timeout")
		}
		assert.Greater(t, callCount, 0, "The mock operation should have been called at least once")
	})

	t.Run("operation_fails_immediately_on_non_retryable_error", func(t *testing.T) {
		// --- Setup ---
		operationName := "testNonRetryableError"
		callCount := 0
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			callCount++
			return nil, nil, http.StatusBadRequest
		}

		// --- Act ---
		retryConfig := CreateRetryConfig(operationName, mockOperation, successCodes, 120, testInitialDelay, testRetryDelay)
		_, err := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.Error(t, err, "RetryOperation should return an error for a non-retryable status")
		if err != nil {
			assert.Contains(t, err.Error(), "non-retryable", "The error message should indicate a non-retryable error")
		}
		assert.Equal(t, 1, callCount, "The mock operation should have been called only once")
	})
}
