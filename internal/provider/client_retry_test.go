package provider

import (
	"context"
	"net/http"
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
				return context.WithTimeout(context.Background(), 20*time.Second)
			},
			expectedTimeout: 16,
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
				// 6.25s * 0.8 = 5s
				return context.WithTimeout(context.Background(), 6250*time.Millisecond)
			},
			expectedTimeout: minTimeoutSeconds,
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

	t.Run("operation succeeds on the first attempt", func(t *testing.T) {
		ctx := context.Background()

		expectedBody := []byte(`{"message": "operation successful"}`)
		operationName := "testSuccessOperation"
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			return expectedBody, nil, http.StatusOK
		}
		successCodes := []int{http.StatusOK}

		// --- Act ---
		result, err := RetryOperation(ctx, operationName, mockOperation, successCodes, testInitialDelay, testRetryDelay)

		// --- Assert ---
		assert.NoError(t, err, "RetryOperation should not return an error on a successful operation")
		assert.Equal(t, expectedBody, result, "The result body should match the one returned by the successful operation")
	})

	t.Run("operation succeeds after a conflict", func(t *testing.T) {
		// --- Setup ---
		// Add a timeout to the context to ensure the test doesn't hang.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

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
		successCodes := []int{http.StatusOK}

		// --- Act ---
		startTime := time.Now()
		result, err := RetryOperation(ctx, operationName, mockOperation, successCodes, testInitialDelay, testRetryDelay)
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
		// Create a context with a short timeout for this test.
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		operationName := "testConflictThatAlwaysFails"
		callCount := 0
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			callCount++
			return nil, nil, http.StatusConflict
		}
		successCodes := []int{http.StatusOK}

		// --- Act ---
		_, err := RetryOperation(ctx, operationName, mockOperation, successCodes, testInitialDelay, testRetryDelay)

		// --- Assert ---
		assert.Error(t, err, "RetryOperation should return an error when it times out")
		if err != nil {
			assert.Contains(t, err.Error(), "timeout", "The error message should indicate a timeout")
		}
		assert.Greater(t, callCount, 0, "The mock operation should have been called at least once")
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
		successCodes := []int{http.StatusOK}

		// --- Act ---
		_, err := RetryOperation(ctx, operationName, mockOperation, successCodes, testInitialDelay, testRetryDelay)

		// --- Assert ---
		assert.Error(t, err, "RetryOperation should return an error for a non-retryable status")
		if err != nil {
			assert.Contains(t, err.Error(), "non-retryable", "The error message should indicate a non-retryable error")
		}
		assert.Equal(t, 1, callCount, "The mock operation should have been called only once")
	})
}
