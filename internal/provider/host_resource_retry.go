package provider

import (
	"context"
	"fmt"
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

// Default Retry Time Constants
const (
	defaultRetryTimeout     = 1800 // Overall timeout for retry (seconds) Default: 30min
	defaultRetryDelay       = 5    // Time to wait between retries (seconds)
	defaultRetryInitalDelay = 2    // Initial delay before first retry (seconds)
)

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
func CreateRetryConfig(operationName string, operation HostOperationFunc, successStatusCodes []int,
	retryableStatusCodes []int, retryTimeout int64, initialDelay int64, retryDelay int64,
) *HostRetryConfig {
	// Use provided delays, fallback to defaults if zero
	if retryTimeout == 0 {
		retryTimeout = defaultRetryTimeout
	}
	if initialDelay == 0 {
		initialDelay = defaultRetryDelay
	}
	if retryDelay == 0 {
		retryDelay = defaultRetryInitalDelay
	}

	stateConf := &retry.StateChangeConf{
		Pending: []string{hostRetryStateRetrying},
		Target:  []string{hostRetryStateSuccess},
		Refresh: func() (interface{}, string, error) {
			body, diags, statusCode := operation()

			// Check for retryable status codes
			for _, retryableCode := range retryableStatusCodes {
				if statusCode == retryableCode {
					return nil, hostRetryStateRetrying, nil // Keep retrying
				}
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
		MinTimeout: time.Duration(retryDelay) * time.Second,
		Delay:      time.Duration(initialDelay) * time.Second,
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
