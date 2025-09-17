//go:generate mockgen -source=retry.go -destination=mock_provider/mock_retry.go
package provider

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// RetryOperationFunc defines the signature for operations that can be retried
type RetryOperationFunc func() ([]byte, diag.Diagnostics, int)

// RetryOperation interface for testing with mocks
type RetryOperation interface {
	Execute() ([]byte, diag.Diagnostics, int)
}

// WrapRetryOperation converts a RetryOperation interface to a RetryOperationFunc
func WrapRetryOperation(op RetryOperation) RetryOperationFunc {
	return func() ([]byte, diag.Diagnostics, int) {
		return op.Execute()
	}
}

// RetryConfig contains configuration for retrying host operations
type RetryConfig struct {
	stateConf          *retry.StateChangeConf
	operationName      string
	operation          RetryOperationFunc
	successStatusCodes []int
	ctx                context.Context
}

type RetryResult struct {
	Body  []byte
	Diags diag.Diagnostics
	State string
}

const (
	// Default Retry Times
	DefaultRetryTimeout      = 1800 // Overall timeout for retry (seconds) Default: 30min
	DefaultRetryDelay        = 5    // Time to wait between retries (seconds)
	DefaultRetryInitialDelay = 2    // Initial delay before first retry (seconds)

	// Retry States
	RetryStateError    = "error"
	RetryStateRetrying = "retrying"
	RetryStateSuccess  = "success"
)

var (
	// Success status codes for retry operations
	DefaultRetrySuccessStatusCodes = []int{http.StatusAccepted, http.StatusNoContent}

	// Retryable status codes for retry operations
	// Common retryable scenarios based on RFC 7231 and industry standards:
	// - HTTP 409: Resource conflict (host in use by running jobs)
	// - HTTP 408: Request timeout
	// - HTTP 429: Too many requests (rate limiting)
	// - HTTP 500: Internal server error
	// - HTTP 502: Bad gateway
	// - HTTP 503: Service unavailable
	// - HTTP 504: Gateway timeout
	//
	// HTTP 403: Forbidden
	// Acceptance tests run against AAP 2.4 almost always receives a
	// 403 upon first host deletion attempt. This is likely an invalid
	// response code from AAP 2.4 and the real error is not known.
	DefaultRetryableStatusCodes = []int{http.StatusConflict, http.StatusRequestTimeout,
		http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusForbidden}
)

// SafeDurationFromSeconds safely converts seconds to time.Duration, checking for overflow
func SafeDurationFromSeconds(seconds int64) (time.Duration, error) {
	// Maximum duration in seconds for int64 is roughly 292 years
	const maxDurationSeconds = math.MaxInt64 / int64(time.Second)
	if seconds < 0 {
		return 0, fmt.Errorf("duration must be non-negative, got: %d seconds", seconds)
	}
	if seconds > maxDurationSeconds {
		return 0, fmt.Errorf("duration overflow: %d seconds exceeds maximum allowed duration", seconds)
	}

	return time.Duration(seconds) * time.Second, nil
}

// CreateRetryConfig creates a RetryConfig wrapping Terraform's retry.StateChangeConf object
func CreateRetryConfig(ctx context.Context, operationName string, operation RetryOperationFunc,
	successStatusCodes []int, retryableStatusCodes []int, retryTimeout int64, initialDelay int64,
	retryDelay int64) (*RetryConfig, diag.Diagnostics) {
	const unableRetryMsg = "Unable to retry"
	var diags diag.Diagnostics

	if operation == nil {
		diags.AddError(
			"Error configuring retry",
			"Retry function is not defined",
		)
		return nil, diags
	}

	if len(successStatusCodes) == 0 {
		successStatusCodes = DefaultRetrySuccessStatusCodes
	}
	if len(retryableStatusCodes) == 0 {
		retryableStatusCodes = DefaultRetryableStatusCodes
	}

	// Check for overflow when converting to time.Duration
	timeoutDuration, err := SafeDurationFromSeconds(retryTimeout)
	if err != nil {
		diags.AddError(
			unableRetryMsg,
			fmt.Sprintf("invalid retry timeout: %s", err.Error()),
		)
	}
	retryDelayDuration, err := SafeDurationFromSeconds(retryDelay)
	if err != nil {
		diags.AddError(
			unableRetryMsg,
			fmt.Sprintf("invalid retry delay: %s", err.Error()),
		)
	}
	initialDelayDuration, err := SafeDurationFromSeconds(initialDelay)
	if err != nil {
		diags.AddError(
			unableRetryMsg,
			fmt.Sprintf("invalid initial delay: %s", err.Error()))
	}
	if diags.HasError() {
		return nil, diags
	}

	result := &RetryResult{}
	stateConf := &retry.StateChangeConf{
		Pending: []string{RetryStateRetrying},
		Target:  []string{RetryStateSuccess},
		Refresh: func() (interface{}, string, error) {
			body, diags, statusCode := operation()
			result.Body = body

			if slices.Contains(retryableStatusCodes, statusCode) {
				result.State = RetryStateRetrying
				return result, RetryStateRetrying, nil // Keep retrying
			}
			if slices.Contains(successStatusCodes, statusCode) {
				result.State = RetryStateSuccess
				return result, RetryStateSuccess, nil
			}

			// If status code is not retryable append the error returned
			result.Diags.Append(diags...)
			return result, RetryStateError, fmt.Errorf("non-retryable HTTP status %d for %s", statusCode, operationName)
		},
		Timeout:    timeoutDuration,
		MinTimeout: retryDelayDuration,
		Delay:      initialDelayDuration,
	}

	return &RetryConfig{
		stateConf:          stateConf,
		operationName:      operationName,
		operation:          operation,
		successStatusCodes: successStatusCodes,
		ctx:                ctx,
	}, diags
}

// RetryWithConfig executes a retry operation with the provided configuration
func RetryWithConfig(retryConfig *RetryConfig) (*RetryResult, error) {
	if retryConfig == nil {
		return nil, fmt.Errorf("retry configuration cannot be nil")
	}
	if retryConfig.stateConf == nil {
		return nil, fmt.Errorf("retry operation '%s': state configuration is not initialized", retryConfig.operationName)
	}
	if retryConfig.ctx == nil {
		return nil, fmt.Errorf("retry operation '%s': context cannot be nil", retryConfig.operationName)
	}
	if !IsContextActive(retryConfig.operationName, retryConfig.ctx, nil) {
		return nil, fmt.Errorf("retry operation '%s': context is not active", retryConfig.operationName)
	}

	result, err := retryConfig.stateConf.WaitForStateContext(retryConfig.ctx)
	if err != nil {
		return nil, fmt.Errorf("retry operation '%s' failed: %w", retryConfig.operationName, err)
	}

	if retryresult, ok := result.(*RetryResult); ok {
		if retryresult.Diags.HasError() && retryresult.State != RetryStateError {
			return retryresult, fmt.Errorf("retry operation '%s' returned errors with retry state '%s'", retryConfig.operationName, retryresult.State)
		}
		return retryresult, nil
	}

	return nil, fmt.Errorf("retry operation '%s' returned unexpected result type: %T (expected *RetryResult)", retryConfig.operationName, result)
}
