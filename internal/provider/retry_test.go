package provider

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/ansible/terraform-provider-aap/internal/provider/mock_provider"
)

// TestRetryOperation tests the retry functionality with defensive programming.
// Key behavior: HTTP success status codes take precedence over diagnostic errors.
// Diagnostic errors are only processed and returned when HTTP status codes are non-retryable.
func TestRetryOperation(t *testing.T) {
	t.Parallel()
	testSetup := func(t *testing.T) (context.Context, *gomock.Controller, []int, []int, int64, int64, int64) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()
		successCodes := []int{http.StatusOK}
		retryableCodes := []int{http.StatusConflict}
		testTimeout := int64(10)
		testInitialDelay := int64(1)
		testRetryDelay := int64(1)

		return ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay
	}

	t.Run("operation succeeds on the first attempt", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		expectedBody := []byte(`{"message": "operation successful"}`)
		operationName := "testSuccessOperation"
		mockOperation := func() ([]byte, diag.Diagnostics, int) {
			return expectedBody, nil, http.StatusOK
		}

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, mockOperation, successCodes, retryableCodes,
			testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryWithConfig should not return an error on a successful operation")
		assert.Equal(t, expectedBody, result.Body, "The result body should match the one returned by the successful operation")
	})

	t.Run("operation succeeds after a conflict", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		expectedBody := []byte(`{"message": "operation eventually successful"}`)
		operationName := "testConflictThenSuccessOperation"

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		gomock.InOrder(
			mockOp.EXPECT().Execute().Return(nil, diag.Diagnostics{}, http.StatusConflict),
			mockOp.EXPECT().Execute().Return(expectedBody, diag.Diagnostics{}, http.StatusOK),
		)

		// --- Act ---
		startTime := time.Now()
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes, retryableCodes,
			testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)
		elapsedTime := time.Since(startTime)

		// --- Assert ---
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should not return an error on eventual success")
		assert.Equal(t, expectedBody, result.Body, "The result body should match the one from the successful call")

		// More lenient timing assertion with tolerance for system overhead
		expectedMinDuration := time.Duration(testInitialDelay) * time.Second
		tolerance := 100 * time.Millisecond // Allow for execution overhead
		assert.GreaterOrEqual(t, elapsedTime, expectedMinDuration-tolerance,
			"The elapsed time should be at least the initial delay (with tolerance for overhead)")
	})

	t.Run("operation_fails_immediately_on_non_retryable_error", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		operationName := "testNonRetryableError"

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		mockOp.EXPECT().Execute().Return(nil, diag.Diagnostics{}, http.StatusBadRequest).Times(1)

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes, retryableCodes,
			testTimeout, testInitialDelay, testRetryDelay)
		_, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.Error(t, err2, "RetryOperation should return an error for a non-retryable status")
		if err2 != nil {
			assert.Contains(t, err2.Error(), "non-retryable", "The error message should indicate a non-retryable error")
		}
	})

	t.Run("operation times out after multiple retries", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		operationName := "testTimeoutOperation"

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		// Expect at least 2 calls, but allow more since timing can vary
		mockOp.EXPECT().Execute().Return(nil, diag.Diagnostics{}, http.StatusConflict).MinTimes(2).MaxTimes(5)

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes, retryableCodes,
			testTimeout, testInitialDelay, testRetryDelay)
		_, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.Error(t, err2, "RetryOperation should timeout with continuous retryable errors")
	})

	t.Run("operation with multiple retryable status codes", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, _, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		// Test different retryable status codes
		operationName := "testMultipleRetryableStatusCodes"
		extendedRetryableCodes := []int{http.StatusConflict, http.StatusTooManyRequests, http.StatusServiceUnavailable}
		expectedBody := []byte(`{"message": "success after multiple retries"}`)

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		gomock.InOrder(
			mockOp.EXPECT().Execute().Return(nil, diag.Diagnostics{}, http.StatusTooManyRequests),
			mockOp.EXPECT().Execute().Return(nil, diag.Diagnostics{}, http.StatusServiceUnavailable),
			mockOp.EXPECT().Execute().Return(nil, diag.Diagnostics{}, http.StatusConflict),
			mockOp.EXPECT().Execute().Return(expectedBody, diag.Diagnostics{}, http.StatusOK),
		)

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes,
			extendedRetryableCodes, testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should succeed after multiple different retryable errors")
		assert.Equal(t, expectedBody, result.Body, "Should return expected body after retries")
	})

	t.Run("operation with multiple success status codes", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, _, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		operationName := "testMultipleSuccessStatusCodes"
		expectedBody := []byte(`{"message": "accepted operation"}`)
		extendedSuccessCodes := []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent}

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		mockOp.EXPECT().Execute().Return(expectedBody, diag.Diagnostics{}, http.StatusAccepted).Times(1)

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), extendedSuccessCodes,
			retryableCodes, testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should succeed with 202 Accepted status")
		assert.Equal(t, expectedBody, result.Body, "Should return expected body for accepted status")
	})

	t.Run("operation succeeds when all HTTP status codes are retryable", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		operationName := "testSuccessWithDiagnosticErrors"
		var diags diag.Diagnostics
		diags.AddError("Test Error", "This is a test diagnostic error")

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		mockOp.EXPECT().Execute().Return([]byte("test"), diags, http.StatusOK).Times(1)

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes,
			retryableCodes, testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		// With defensive programming, HTTP success status codes take precedence over diagnostic errors
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should succeed when all HTTP status codes are retryable")
		assert.NotNil(t, result, "Result should be returned when RetryOperation succeeds")
		assert.Equal(t, []byte("test"), result.Body, "Result body should match the returned value")
	})

	t.Run("operation fails with diagnostic errors and non-retryable HTTP status code", func(t *testing.T) {
		// --- Setup ---
		t.Parallel()
		ctx, ctrl, successCodes, retryableCodes, testTimeout, testInitialDelay, testRetryDelay := testSetup(t)
		defer ctrl.Finish()

		operationName := "testFailureWithDiagnosticErrors"
		var diags diag.Diagnostics
		diags.AddError("Test Error", "This is a test diagnostic error")

		mockOp := mock_provider.NewMockRetryOperation(ctrl)
		// Use a non-retryable, non-success status code (400 Bad Request)
		mockOp.EXPECT().Execute().Return([]byte("error response"), diags, http.StatusBadRequest).Times(1)

		// --- Act ---
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes,
			retryableCodes, testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		// With defensive programming, diagnostics are appended when status is non-retryable
		assert.False(t, err1.HasError(), "CreateRetryConfig should not return an error")
		assert.Error(t, err2, "RetryOperation should return an error for non-retryable status codes")
		assert.Nil(t, result, "Result should be nil when retry operation fails")
		if err2 != nil {
			assert.Contains(t, err2.Error(), "non-retryable HTTP status 400", "Error message should indicate non-retryable status")
		}
	})
}

// validateBasicRetryConfig validates common config fields
func validateBasicRetryConfig(t *testing.T, config *RetryConfig, operationName string, ctx context.Context,
	expectedSuccessCodes []int) {
	assert.Equal(t, operationName, config.operationName)
	assert.Equal(t, ctx, config.ctx)
	assert.Equal(t, expectedSuccessCodes, config.successStatusCodes)
	assert.NotNil(t, config.stateConf)
	assert.NotNil(t, config.operation)
}

// validateRetryStateConf validates StateChangeConf fields
func validateRetryStateConf(t *testing.T, config *RetryConfig, expectedTimeout, expectedDelay,
	expectedMinTimeout time.Duration) {
	assert.Equal(t, []string{RetryStateRetrying}, config.stateConf.Pending)
	assert.Equal(t, []string{RetryStateSuccess}, config.stateConf.Target)
	assert.Equal(t, expectedTimeout, config.stateConf.Timeout)
	assert.Equal(t, expectedDelay, config.stateConf.Delay)
	assert.Equal(t, expectedMinTimeout, config.stateConf.MinTimeout)
	assert.NotNil(t, config.stateConf.Refresh)
}

func TestCreateRetryConfig(t *testing.T) {
	const operationName = "testOperation"
	ctx := context.Background()
	mockOperation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("test"), nil, http.StatusOK
	}
	successCodes := []int{http.StatusOK}
	retryableCodes := []int{http.StatusConflict}
	maxDurationSeconds := math.MaxInt64 / int64(time.Second)
	overflowValue := maxDurationSeconds + 1

	t.Run("error cases", func(t *testing.T) {
		errorTests := []struct {
			name          string
			operation     RetryOperationFunc
			timeout       int64
			initialDelay  int64
			retryDelay    int64
			errorContains string
		}{
			{"nil operation", nil, 120, 2, 5, "Retry function is not defined"},
			{"overflow timeout", mockOperation, overflowValue, 2, 5, "invalid retry timeout"},
			{"overflow initial delay", mockOperation, 120, overflowValue, 5, "invalid initial delay"},
			{"overflow retry delay", mockOperation, 120, 2, overflowValue, "invalid retry delay"},
			{"negative timeout", mockOperation, -1, 2, 5, "invalid retry timeout"},
			{"all overflow values", mockOperation, overflowValue, overflowValue, overflowValue, "invalid retry timeout"},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				config, err := CreateRetryConfig(ctx, operationName, tt.operation, successCodes, retryableCodes,
					tt.timeout, tt.initialDelay, tt.retryDelay)

				assert.True(t, err.HasError())
				assert.Nil(t, config)
				found := false
				for _, e := range err.Errors() {
					if strings.Contains(e.Summary(), tt.errorContains) || strings.Contains(e.Detail(), tt.errorContains) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected to find '%s' in error messages: %v", tt.errorContains, err.Errors())
			})
		}
	})

	t.Run("default status codes", func(t *testing.T) {
		defaultTests := []struct {
			name            string
			successCodes    []int
			retryableCodes  []int
			expectedSuccess []int
		}{
			{"success codes nil", nil, retryableCodes, DefaultRetrySuccessStatusCodes},
			{"retryable codes nil", successCodes, nil, successCodes},
			{"both nil", nil, nil, DefaultRetrySuccessStatusCodes},
			{"both empty", []int{}, []int{}, DefaultRetrySuccessStatusCodes},
		}

		for _, tt := range defaultTests {
			t.Run(tt.name, func(t *testing.T) {
				config, err := CreateRetryConfig(ctx, operationName, mockOperation, tt.successCodes, tt.retryableCodes,
					120, 2, 5)

				assert.False(t, err.HasError())
				assert.NotNil(t, config)
				validateBasicRetryConfig(t, config, operationName, ctx, tt.expectedSuccess)
			})
		}
	})

	t.Run("valid configuration", func(t *testing.T) {
		config, err := CreateRetryConfig(ctx, operationName, mockOperation, successCodes, retryableCodes,
			300, 3, 7)

		assert.False(t, err.HasError())
		assert.NotNil(t, config)
		validateBasicRetryConfig(t, config, operationName, ctx, successCodes)
		validateRetryStateConf(t, config, 300*time.Second, 3*time.Second, 7*time.Second)
	})
}

func TestSafeDurationFromSeconds(t *testing.T) {
	maxDurationSeconds := math.MaxInt64 / int64(time.Second)

	t.Run("success cases", func(t *testing.T) {
		successTests := []struct {
			name             string
			seconds          int64
			expectedDuration time.Duration
		}{
			{"zero seconds", 0, 0},
			{"one second", 1, time.Second},
			{"one minute", 60, time.Minute},
			{"one hour", 3600, time.Hour},
			{"max valid duration", maxDurationSeconds, time.Duration(maxDurationSeconds) * time.Second},
			{"max valid minus one", maxDurationSeconds - 1, time.Duration(maxDurationSeconds-1) * time.Second},
		}

		for _, tt := range successTests {
			t.Run(tt.name, func(t *testing.T) {
				duration, err := SafeDurationFromSeconds(tt.seconds)

				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDuration, duration)
			})
		}
	})

	t.Run("error cases", func(t *testing.T) {
		errorTests := []struct {
			name          string
			seconds       int64
			errorContains string
		}{
			{"negative value", -1, "duration must be non-negative"},
			{"large negative", -100, "duration must be non-negative"},
			{"overflow", maxDurationSeconds + 1, "duration overflow"},
			{"large overflow", math.MaxInt64, "duration overflow"},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				duration, err := SafeDurationFromSeconds(tt.seconds)

				assert.Error(t, err)
				assert.Equal(t, time.Duration(0), duration)
				assert.Contains(t, err.Error(), tt.errorContains)
			})
		}
	})
}

func TestRetryWithConfig(t *testing.T) {
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately
	tests := []struct {
		name                 string
		retryConfig          *RetryConfig
		expectedResult       *RetryResult
		expectedErrorMessage string
	}{
		{
			name: "successfully returns retry result",
			retryConfig: &RetryConfig{
				stateConf: &retry.StateChangeConf{
					Pending: []string{RetryStateRetrying},
					Target:  []string{RetryStateSuccess},
					Refresh: func() (any, string, error) {
						result := &RetryResult{
							Body:  []byte("success result"),
							Diags: diag.Diagnostics{},
							State: RetryStateSuccess,
						}
						return result, RetryStateSuccess, nil
					},
					Timeout:    20 * time.Millisecond,
					MinTimeout: 1 * time.Millisecond,
					Delay:      2 * time.Millisecond,
				},
				operationName: "testOperation",
				ctx:           context.Background(),
			},
			expectedResult: &RetryResult{
				Body:  []byte("success result"),
				Diags: diag.Diagnostics{},
				State: RetryStateSuccess,
			},
			expectedErrorMessage: "",
		},
		{
			name: "returns error when operation has diagnostic errors",
			retryConfig: &RetryConfig{
				stateConf: &retry.StateChangeConf{
					Pending: []string{RetryStateRetrying},
					Target:  []string{RetryStateSuccess},
					Refresh: func() (any, string, error) {
						result := &RetryResult{
							Body:  []byte("test"),
							Diags: diag.Diagnostics{},
							State: RetryStateError,
						}
						return result, RetryStateError, fmt.Errorf("testOperation error occurred during retry operation: test error")
					},
					Timeout:    20 * time.Millisecond,
					MinTimeout: 1 * time.Millisecond,
					Delay:      2 * time.Millisecond,
				},
				operationName: "testOperation",
				ctx:           context.Background(),
			},
			expectedResult:       nil,
			expectedErrorMessage: "error occurred during retry operation",
		},
		{
			name:                 "returns error when retry config is nil",
			retryConfig:          nil,
			expectedResult:       nil,
			expectedErrorMessage: "retry configuration cannot be nil",
		},
		{
			name: "returns error when state config is nil",
			retryConfig: &RetryConfig{
				stateConf:     nil,
				operationName: "testOperation",
				ctx:           context.Background(),
			},
			expectedResult:       nil,
			expectedErrorMessage: "state configuration is not initialized",
		},
		{
			name: "returns error when context is nil",
			retryConfig: &RetryConfig{
				stateConf:     &retry.StateChangeConf{},
				operationName: "testOperation",
				ctx:           nil,
			},
			expectedResult:       nil,
			expectedErrorMessage: "context cannot be nil",
		},
		{
			name: "returns error when context is canceled",
			retryConfig: &RetryConfig{
				stateConf: &retry.StateChangeConf{
					Pending: []string{RetryStateRetrying},
					Target:  []string{RetryStateSuccess},
					Refresh: func() (any, string, error) {
						return []byte("test"), RetryStateSuccess, nil
					},
					Timeout:    20 * time.Millisecond,
					MinTimeout: 1 * time.Millisecond,
					Delay:      2 * time.Millisecond,
				},
				operationName: "testOperation",
				ctx:           cancelledCtx,
			},
			expectedResult:       nil,
			expectedErrorMessage: "context is not active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RetryWithConfig(tt.retryConfig)

			if tt.expectedErrorMessage != "" {
				// Expecting an error
				assert.Error(t, err, "Expected an error but got none")
				assert.Contains(t, err.Error(), tt.expectedErrorMessage, "Error message should contain expected text")
				assert.Equal(t, tt.expectedResult, result, "Result should match expected (likely nil)")
			} else {
				// Expecting success
				assert.NoError(t, err, "Expected no error but got: %v", err)
				assert.NotNil(t, result, "Result should not be nil")
				assert.Equal(t, tt.expectedResult.Body, result.Body, "Result body should match expected")
				assert.Equal(t, tt.expectedResult.State, result.State, "Result state should match expected")
			}
		})
	}
}
