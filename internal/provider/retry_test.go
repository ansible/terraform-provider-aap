package provider

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mock_provider "github.com/ansible/terraform-provider-aap/internal/provider/mocks"
)

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
		assert.NoError(t, err1, "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryWithConfig should not return an error on a successful operation")
		assert.Equal(t, expectedBody, result, "The result body should match the one returned by the successful operation")
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
		assert.NoError(t, err1, "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should not return an error on eventual success")
		assert.Equal(t, expectedBody, result, "The result body should match the one from the successful call")

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
		assert.NoError(t, err1, "CreateRetryConfig should not return an error")
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
		assert.NoError(t, err1, "CreateRetryConfig should not return an error")
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
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), successCodes, extendedRetryableCodes,
			testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.NoError(t, err1, "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should succeed after multiple different retryable errors")
		assert.Equal(t, expectedBody, result, "Should return expected body after retries")
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
		retryConfig, err1 := CreateRetryConfig(ctx, operationName, WrapRetryOperation(mockOp), extendedSuccessCodes, retryableCodes,
			testTimeout, testInitialDelay, testRetryDelay)
		result, err2 := RetryWithConfig(retryConfig)

		// --- Assert ---
		assert.NoError(t, err1, "CreateRetryConfig should not return an error")
		assert.NoError(t, err2, "RetryOperation should succeed with 202 Accepted status")
		assert.Equal(t, expectedBody, result, "Should return expected body for accepted status")
	})
}

// validateBasicRetryConfig validates common config fields
func validateBasicRetryConfig(t *testing.T, config *RetryConfig, operationName string, ctx context.Context, expectedSuccessCodes []int) {
	assert.Equal(t, operationName, config.operationName)
	assert.Equal(t, ctx, config.ctx)
	assert.Equal(t, expectedSuccessCodes, config.successStatusCodes)
	assert.NotNil(t, config.stateConf)
	assert.NotNil(t, config.operation)
}

// validateRetryStateConf validates StateChangeConf fields
func validateRetryStateConf(t *testing.T, config *RetryConfig, expectedTimeout, expectedDelay, expectedMinTimeout time.Duration) {
	assert.Equal(t, []string{RetryStateRetrying}, config.stateConf.Pending)
	assert.Equal(t, []string{RetryStateSuccess}, config.stateConf.Target)
	assert.Equal(t, expectedTimeout, config.stateConf.Timeout)
	assert.Equal(t, expectedDelay, config.stateConf.Delay)
	assert.Equal(t, expectedMinTimeout, config.stateConf.MinTimeout)
	assert.NotNil(t, config.stateConf.Refresh)
}

// validateRetryDefaults validates that all default values are applied correctly
func validateRetryDefaults(t *testing.T, config *RetryConfig, operationName string, ctx context.Context) {
	validateBasicRetryConfig(t, config, operationName, ctx, DefaultRetrySuccessStatusCodes)
	validateRetryStateConf(t, config, time.Duration(DefaultRetryTimeout)*time.Second,
		time.Duration(DefaultRetryDelay)*time.Second, time.Duration(DefaultRetryInitialDelay)*time.Second)
}

func TestCreateRetryConfig(t *testing.T) {
	const operationName = "testOperation"
	ctx := context.Background()
	mockOperation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("test"), nil, http.StatusOK
	}
	successCodes := []int{http.StatusOK}
	retryableCodes := []int{http.StatusConflict}

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
			errorContains:  "retry function is not defined",
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
			name:           "applies all defaults when parameters are zero or nil",
			operation:      mockOperation,
			successCodes:   nil,
			retryableCodes: []int{},
			timeout:        0,
			initialDelay:   0,
			retryDelay:     0,
			expectError:    false,
			validateFunc: func(t *testing.T, config *RetryConfig) {
				validateRetryDefaults(t, config, operationName, ctx)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := CreateRetryConfig(ctx, operationName, tt.operation, tt.successCodes, tt.retryableCodes,
				tt.timeout, tt.initialDelay, tt.retryDelay)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
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
		{
			name:             "converts valid positive seconds",
			seconds:          60,
			expectedDuration: time.Minute,
			expectError:      false,
		},
		{
			name:             "converts zero seconds",
			seconds:          0,
			expectedDuration: time.Duration(0),
			expectError:      false,
		},
		{
			name:             "handles maximum valid duration",
			seconds:          maxDurationSeconds,
			expectedDuration: time.Duration(maxDurationSeconds) * time.Second,
			expectError:      false,
		},
		{
			name:             "returns error for negative seconds",
			seconds:          -1,
			expectedDuration: time.Duration(0),
			expectError:      true,
			errorContains:    "duration must be non-negative",
		},
		{
			name:             "returns error for overflow",
			seconds:          maxDurationSeconds + 1,
			expectedDuration: time.Duration(0),
			expectError:      true,
			errorContains:    "duration overflow",
		},
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

func TestCreateRetryConfigOverflow(t *testing.T) {
	ctx := context.Background()
	operationName := "testOperation"
	mockOperation := func() ([]byte, diag.Diagnostics, int) {
		return []byte("test"), nil, http.StatusOK
	}
	successCodes := []int{http.StatusOK}
	retryableCodes := []int{http.StatusConflict}
	maxDurationSeconds := math.MaxInt64 / int64(time.Second)
	overflowValue := maxDurationSeconds + 1

	tests := []struct {
		name            string
		timeout         int64
		initialDelay    int64
		retryDelay      int64
		expectError     bool
		expectedErrors  []string
		testDescription string
	}{
		{
			name:            "returns errors for all overflow values",
			timeout:         overflowValue,
			initialDelay:    overflowValue,
			retryDelay:      overflowValue,
			expectError:     true,
			expectedErrors:  []string{"invalid retry timeout", "duration overflow"},
			testDescription: "Tests timeout overflow (first parameter checked)",
		},
		{
			name:            "returns errors for all negative values",
			timeout:         -1,
			initialDelay:    -1,
			retryDelay:      -1,
			expectError:     true,
			expectedErrors:  []string{"invalid retry timeout", "duration must be non-negative"},
			testDescription: "Tests negative timeout (first parameter checked)",
		},
		{
			name:            "returns error for initial delay overflow when timeout is valid",
			timeout:         120,
			initialDelay:    overflowValue,
			retryDelay:      5,
			expectError:     true,
			expectedErrors:  []string{"invalid initial delay", "duration overflow"},
			testDescription: "Tests initial delay overflow when timeout passes validation",
		},
		{
			name:            "returns error for retry delay overflow when timeout and initial delay are valid",
			timeout:         120,
			initialDelay:    2,
			retryDelay:      overflowValue,
			expectError:     true,
			expectedErrors:  []string{"invalid retry delay", "duration overflow"},
			testDescription: "Tests retry delay overflow when other parameters pass validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := CreateRetryConfig(ctx, operationName, mockOperation, successCodes, retryableCodes,
				tt.timeout, tt.initialDelay, tt.retryDelay)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
				for _, errorStr := range tt.expectedErrors {
					assert.Contains(t, err.Error(), errorStr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
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
		expectedResult       []byte
		expectedErrorMessage string
	}{
		{
			name: "successfully returns byte array result",
			retryConfig: &RetryConfig{
				stateConf: &retry.StateChangeConf{
					Pending: []string{RetryStateRetrying},
					Target:  []string{RetryStateSuccess},
					Refresh: func() (any, string, error) {
						return []byte("success result"), RetryStateSuccess, nil
					},
					Timeout:    20 * time.Millisecond,
					MinTimeout: 1 * time.Millisecond,
					Delay:      2 * time.Millisecond,
				},
				operationName: "testOperation",
				ctx:           context.Background(),
			},
			expectedResult:       []byte("success result"),
			expectedErrorMessage: "",
		},
		{
			name: "returns error when operation succeeds but has diagnostic errors",
			retryConfig: &RetryConfig{
				stateConf: &retry.StateChangeConf{
					Pending: []string{RetryStateRetrying},
					Target:  []string{RetryStateSuccess},
					Refresh: func() (any, string, error) {
						return nil, "", fmt.Errorf("testOperation succeeded but diagnostics has errors: test error")
					},
					Timeout:    20 * time.Millisecond,
					MinTimeout: 1 * time.Millisecond,
					Delay:      2 * time.Millisecond,
				},
				operationName: "testOperation",
				ctx:           context.Background(),
			},
			expectedResult:       nil,
			expectedErrorMessage: "succeeded but diagnostics has errors",
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
				assert.Equal(t, tt.expectedResult, result, "Result should match expected")
			}
		})
	}
}
