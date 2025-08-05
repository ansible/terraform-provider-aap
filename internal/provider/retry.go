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
//go:generate mockgen -source=retry.go -destination=mocks/mock_retry.go
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

const (
	// Default Retry Times
	DefaultRetryTimeout      = 1800 // Overall timeout for retry (seconds) Default: 30min
	DefaultRetryDelay        = 5    // Time to wait between retries (seconds)
	DefaultRetryInitialDelay = 2    // Initial delay before first retry (seconds)

	// Retry States
	RetryStateRetrying = "retrying"
	RetryStateSuccess  = "success"
)

var (
	// Success status codes for delete operation
	DefaultRetrySuccessStatusCodes = []int{http.StatusAccepted, http.StatusNoContent}

	// Retryable status codes for host delete operations
	DefaultRetryableStatusCodes = []int{
		http.StatusConflict,
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
)

// SafeDurationFromSeconds safely converts seconds to time.Duration, checking for overflow
// and validating against protobuf duration constraints for positive values
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

// CreateRetryConfig creates a StateChangeConf for retrying operations with exponential backoff.
// This follows Terraform provider best practices for handling transient API errors.
//
// Common retryable scenarios based on RFC 7231 and industry standards:
// - HTTP 409: Resource conflict (host in use by running jobs)
// - HTTP 408: Request timeout
// - HTTP 429: Too many requests (rate limiting)
// - HTTP 500: Internal server error
// - HTTP 502: Bad gateway
// - HTTP 503: Service unavailable
// - HTTP 504: Gateway timeout
//
// Uses the provided timeout seconds instead of calculating from context deadline.
func CreateRetryConfig(ctx context.Context, operationName string, operation RetryOperationFunc,
	successStatusCodes []int, retryableStatusCodes []int, retryTimeout int64, initialDelay int64,
	retryDelay int64) (*RetryConfig, error) {
	if operation == nil {
		return nil, fmt.Errorf("retry function is not defined: unable to retry")
	}

	if len(successStatusCodes) == 0 || successStatusCodes == nil {
		successStatusCodes = DefaultRetrySuccessStatusCodes
	}

	if len(retryableStatusCodes) == 0 || retryableStatusCodes == nil {
		retryableStatusCodes = DefaultRetryableStatusCodes
	}

	// Terraform will pass in zero if user leaves values blank in HCL
	if retryTimeout == 0 {
		retryTimeout = DefaultRetryTimeout
	}
	if initialDelay == 0 {
		initialDelay = DefaultRetryDelay
	}
	if retryDelay == 0 {
		retryDelay = DefaultRetryInitialDelay
	}

	// Check for overflow when converting to time.Duration
	timeoutDuration, err := SafeDurationFromSeconds(retryTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid retry timeout: %w", err)
	}

	retryDelayDuration, err := SafeDurationFromSeconds(retryDelay)
	if err != nil {
		return nil, fmt.Errorf("invalid retry delay: %w", err)
	}

	initialDelayDuration, err := SafeDurationFromSeconds(initialDelay)
	if err != nil {
		return nil, fmt.Errorf("invalid initial delay: %w", err)
	}

	stateConf := &retry.StateChangeConf{
		Pending: []string{RetryStateRetrying},
		Target:  []string{RetryStateSuccess},
		Refresh: func() (interface{}, string, error) {
			body, diags, statusCode := operation()

			// Check for retryable status codes
			if slices.Contains(retryableStatusCodes, statusCode) {
				return nil, RetryStateRetrying, nil // Keep retrying
			}

			// Check for success cases
			if slices.Contains(successStatusCodes, statusCode) {
				if diags.HasError() {
					return nil, "", fmt.Errorf("%s succeeded but diagnostics has errors: %v", operationName, diags)
				}
				return body, RetryStateSuccess, nil
			}

			// Non-retryable error
			return nil, "", fmt.Errorf("non-retryable HTTP status %d for %s", statusCode, operationName)
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
	}, nil
}

// RetryWithConfig executes a retry operation with the provided configuration
func RetryWithConfig(retryConfig *RetryConfig) ([]byte, error) {
	result, err := retryConfig.stateConf.WaitForStateContext(retryConfig.ctx)
	if err != nil {
		return nil, err
	}

	if body, ok := result.([]byte); ok {
		return body, nil
	}

	return nil, fmt.Errorf("unexpected result type from successful retry: %T", result)
}
