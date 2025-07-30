package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

// HostOperationFunc defines the signature for operations that can be retried
type HostOperationFunc func() ([]byte, diag.Diagnostics, int)

// HostRetryConfig contains configuration for retrying host operations
type HostRetryConfig struct {
	stateConf          *retry.StateChangeConf
	operationName      string
	operation          HostOperationFunc
	successStatusCodes []int
	ctx                context.Context
}

// Retry state constants
const (
	hostRetryStateRetrying = "retrying"
	hostRetryStateSuccess  = "success"
)

// Retry timing constants
const (
	maxTimeoutSeconds = 30              // Maximum wait between retries (seconds)
	minTimeoutSeconds = 5               // Minimum wait between retries (seconds)
	delaySeconds      = 2               // Initial delay before first retry (seconds)
	defaultBuffer     = 1 * time.Minute // Default buffer to leave between context deadline and timeout
)

// CalculateTimeout returns the retry timeout in seconds, which is 1 minute less than the context timeout.
func CalculateTimeout(operationTimeoutSec int) int {
	// Default fallback timeout in seconds
	var timeout int

	// If the deadline has already passed, use the minimum timeout
	if operationTimeoutSec <= 0 {
		return minTimeoutSeconds
	}

	// Use 80% of the remaining time for the timeout
	calculatedTimeoutSeconds := operationTimeoutSec - int(defaultBuffer.Seconds())

	// Ensure the timeout is at least the minimum viable timeout
	if calculatedTimeoutSeconds < minTimeoutSeconds {
		timeout = minTimeoutSeconds
	} else {
		// Return the timeout as an integer, truncating any fractions of a second
		timeout = calculatedTimeoutSeconds
	}

	return timeout
}

// CreateRetryConfig creates a StateChangeConf for retrying operations with exponential backoff.
// This follows Terraform provider best practices for handling transient API errors.
//
// Retryable scenarios based on RFC 7231 and industry standards:
// - HTTP 409: Resource conflict (host in use by running jobs)
// - HTTP 408/429: Client timeouts and rate limiting
// - HTTP 5xx: Server-side transient errors
//
// The retry timeout is calculated from the context deadline, leaving a buffer to prevent conflicts.
func CreateRetryConfig(
	operationName string,
	operation HostOperationFunc,
	successStatusCodes []int,
	timeoutSeconds int64,
	initialDelay time.Duration,
	retryDelay time.Duration,
) *HostRetryConfig {
	retryTimeout := CalculateTimeout(int(timeoutSeconds))

	// Use provided delays, fallback to defaults if zero
	if initialDelay == 0 {
		initialDelay = delaySeconds * time.Second
	}
	if retryDelay == 0 {
		retryDelay = minTimeoutSeconds * time.Second
	}

	stateConf := &retry.StateChangeConf{
		Pending: []string{hostRetryStateRetrying},
		Target:  []string{hostRetryStateSuccess},
		Refresh: func() (interface{}, string, error) {
			body, diags, statusCode := operation()

			// Check for retryable status codes
			switch statusCode {
			case http.StatusConflict, http.StatusRequestTimeout, http.StatusTooManyRequests,
				http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable,
				http.StatusGatewayTimeout:
				return nil, hostRetryStateRetrying, nil // Keep retrying
			}

			// Check for success cases
			for _, successCode := range successStatusCodes {
				if statusCode == successCode {
					if diags.HasError() {
						return nil, "", fmt.Errorf("%s succeeded but diagnostics has errors: %v", operationName, diags)
					}
					return body, hostRetryStateSuccess, nil
				}
			}

			// Non-retryable error
			return nil, "", fmt.Errorf("non-retryable HTTP status %d for %s", statusCode, operationName)
		},
		Timeout:    time.Duration(retryTimeout) * time.Second,
		MinTimeout: retryDelay,   // Use the provided retry delay
		Delay:      initialDelay, // Use the provided initial delay
	}

	return &HostRetryConfig{
		stateConf:          stateConf,
		operationName:      operationName,
		operation:          operation,
		successStatusCodes: successStatusCodes,
		ctx:                context.Background(),
	}
}

// RetryWithConfig executes a retry operation with the provided configuration
func RetryWithConfig(retryConfig *HostRetryConfig) ([]byte, error) {
	result, err := retryConfig.stateConf.WaitForStateContext(retryConfig.ctx)
	if err != nil {
		return nil, err
	}

	if body, ok := result.([]byte); ok {
		return body, nil
	}

	return nil, fmt.Errorf("unexpected result type from successful retry: %T", result)
}
