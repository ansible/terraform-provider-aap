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

// TestCreateRetryStateChangeConf_Success verifies the success path of the retry logic.
func TestCreateRetryStateChangeConf_Success(t *testing.T) {
	t.Run("operation succeeds on the first attempt", func(t *testing.T) {
		// --- Setup ---
		ctx := context.Background()
		expectedBody := []byte(`{"message": "operation successful"}`)
		operationName := "testSuccessOperation"

		// Define a mock operation that simulates a successful API call
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			// Return the expected body, no diagnostics errors, and a success status code
			return expectedBody, nil, http.StatusOK
		}

		// Define the list of status codes that indicate success
		successCodes := []int{http.StatusOK, http.StatusAccepted}

		// --- Act ---
		// Create the StateChangeConf using the function under test
		stateConf := CreateRetryStateChangeConf(ctx, mockOperation, successCodes, operationName)

		// Directly call the Refresh function to test its logic
		result, state, err := stateConf.Refresh()

		// --- Assert ---
		// Use testify/assert for clear and concise assertions
		assert.NoError(t, err, "Refresh function should not return an error on a successful operation")
		assert.Equal(t, retryStateSuccess, state, "The state should be 'SUCCESS' after a successful operation")
		assert.Equal(t, expectedBody, result, "The result body should match the one returned by the successful operation")
	})
}
