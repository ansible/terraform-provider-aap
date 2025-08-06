//go:generate mockgen -source=retry.go -destination=mock_provider/mock_retry.go
package provider

import (
	"context"
	"fmt"
	"net/http"
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
	DefaultRetryableStatusCodes = []int{http.StatusConflict, http.StatusRequestTimeout,
		http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout}
)

// SafeDurationFromSeconds safely converts seconds to time.Duration, checking for overflow
func SafeDurationFromSeconds(seconds int64) (time.Duration, error) {
	// not implemented
	return time.Duration(1) * time.Second, nil
}

// CreateRetryConfig creates a RetryConfig wrapping Terraform's retry.StateChangeConf object
func CreateRetryConfig(ctx context.Context, operationName string, operation RetryOperationFunc,
	successStatusCodes []int, retryableStatusCodes []int, retryTimeout int64, initialDelay int64,
	retryDelay int64) (*RetryConfig, diag.Diagnostics) {
	// not implemented
	return nil, nil
}

// RetryWithConfig executes a retry operation with the provided configuration
func RetryWithConfig(retryConfig *RetryConfig) (*RetryResult, error) {
	return nil, fmt.Errorf("not implemented")
}
