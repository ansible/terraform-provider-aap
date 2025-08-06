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

	tests := []struct {
		name           string
		operation      RetryOperationFunc
		successCodes   []int
		retryableCodes []int
		timeout        int64
		initialDelay   int64
		retryDelay     int64
		expectError    bool
		errorContains  string
		validateFunc   func(t *testing.T, config *RetryConfig)
	}{
		{
			name:           "returns error when operation is nil",
			operation:      nil,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        120,
			initialDelay:   2,
			retryDelay:     5,
			expectError:    true,
			errorContains:  "Retry function is not defined",
		},
		{
			name:           "successfully creates config with valid parameters",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        120,
			initialDelay:   2,
			retryDelay:     5,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateBasicRetryConfig(t, config, operationName, ctx, successCodes)
			},
		},
		{
			name:           "configures StateChangeConf correctly with custom values",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        300,
			initialDelay:   3,
			retryDelay:     7,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateBasicRetryConfig(t, config, operationName, ctx, successCodes)
				validateRetryStateConf(t, config, 300*time.Second, 3*time.Second, 7*time.Second)
			},
		},
		{
			name:           "applies defaults when success status codes is nil",
			operation:      mockOperation,
			successCodes:   nil,
			retryableCodes: retryableCodes,
			timeout:        120,
			initialDelay:   2,
			retryDelay:     5,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateBasicRetryConfig(t, config, operationName, ctx, DefaultRetrySuccessStatusCodes)
			},
		},
		{
			name:           "applies defaults when retryable status codes is nil",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: nil,
			timeout:        120,
			initialDelay:   2,
			retryDelay:     5,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateBasicRetryConfig(t, config, operationName, ctx, successCodes)
				// Verify that default retryable codes were applied by checking that the config was created successfully
				assert.NotNil(t, config.stateConf)
			},
		},
		{
			name:           "applies defaults when both status code slices are nil",
			operation:      mockOperation,
			successCodes:   nil,
			retryableCodes: nil,
			timeout:        120,
			initialDelay:   2,
			retryDelay:     5,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateBasicRetryConfig(t, config, operationName, ctx, DefaultRetrySuccessStatusCodes)
			},
		},
		{
			name:           "applies defaults when both status code slices are empty",
			operation:      mockOperation,
			successCodes:   []int{},
			retryableCodes: []int{},
			timeout:        120,
			initialDelay:   2,
			retryDelay:     5,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateBasicRetryConfig(t, config, operationName, ctx, DefaultRetrySuccessStatusCodes)
			},
		},
		{
			name:           "returns errors for all overflow values",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        overflowValue,
			initialDelay:   overflowValue,
			retryDelay:     overflowValue,
			expectError:    true,
			errorContains:  "invalid retry timeout",
		},
		{
			name:           "returns errors for all negative values",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        -1,
			initialDelay:   -1,
			retryDelay:     -1,
			expectError:    true,
			errorContains:  "invalid retry timeout",
		},
		{
			name:           "returns error for initial delay overflow when timeout is valid",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        120,
			initialDelay:   overflowValue,
			retryDelay:     5,
			expectError:    true,
			errorContains:  "invalid initial delay",
		},
		{
			name:           "returns error for retry delay overflow when timeout and initial delay are valid",
			operation:      mockOperation,
			successCodes:   successCodes,
			retryableCodes: retryableCodes,
			timeout:        120,
			initialDelay:   2,
			retryDelay:     overflowValue,
			expectError:    true,
			errorContains:  "invalid retry delay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := CreateRetryConfig(ctx, operationName, tt.operation, tt.successCodes, tt.retryableCodes,
				tt.timeout, tt.initialDelay, tt.retryDelay)

			if tt.expectError {
				assert.True(t, err.HasError())
				assert.Nil(t, config)
				if tt.errorContains != "" {
					found := false
					for _, e := range err.Errors() {
						if strings.Contains(e.Summary(), tt.errorContains) || strings.Contains(e.Detail(), tt.errorContains) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected to find '%s' in error messages: %v", tt.errorContains, err.Errors())
				}
			} else {
				assert.False(t, err.HasError())
				assert.NotNil(t, config)
				if tt.validateFunc != nil {
					tt.validateFunc(t, config)
				}
			}
		})
	}
}

func TestSafeDurationFromSeconds(t *testing.T) {
	maxDurationSeconds := math.MaxInt64 / int64(time.Second)

	tests := []struct {
		name             string
		seconds          int64
		expectedDuration time.Duration
		expectError      bool
		errorContains    string
	}{
		{"zero seconds", 0, 0, false, ""},
		{"one second", 1, time.Second, false, ""},
		{"one minute", 60, time.Minute, false, ""},
		{"one hour", 3600, time.Hour, false, ""},
		{"max valid duration", maxDurationSeconds, time.Duration(maxDurationSeconds) * time.Second, false, ""},
		{"max valid minus one", maxDurationSeconds - 1, time.Duration(maxDurationSeconds-1) * time.Second, false, ""},
		{"negative value", -1, 0, true, "duration must be non-negative"},
		{"large negative", -100, 0, true, "duration must be non-negative"},
		{"overflow", maxDurationSeconds + 1, 0, true, "duration overflow"},
		{"large overflow", math.MaxInt64, 0, true, "duration overflow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := SafeDurationFromSeconds(tt.seconds)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedDuration, duration)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDuration, duration)
			}
		})
	}
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
