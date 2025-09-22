package provider

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// AAPClientAuthenticator defines the interface for AAP client authentication methods
type AAPClientAuthenticator interface {
	Configure(*http.Request)
}

// AAPClientBasicAuthenticator supports username/password auth
type AAPClientBasicAuthenticator struct {
	username string
	password string
}

// NewBasicAuthenticator creates a new basic authentication authenticator
func NewBasicAuthenticator(username *string, password *string) (*AAPClientBasicAuthenticator, diag.Diagnostics) {
	var diags diag.Diagnostics
	if username == nil {
		diags.AddError(
			"Missing username",
			"Unable to create a basic authenticator without username")
	}
	if password == nil {
		diags.AddError(
			"Missing password",
			"Unable to create a basic authenticator without password")
	}
	if diags.HasError() {
		return nil, diags
	}
	return &AAPClientBasicAuthenticator{
		username: *username,
		password: *password,
	}, nil
}

// Configure configures the HTTP request with basic authentication
func (a *AAPClientBasicAuthenticator) Configure(req *http.Request) {
	// To configure basic auth, we can just use http.Request's SetBasicAuth
	req.SetBasicAuth(a.username, a.password)
}

// AAPClientTokenAuthenticator supports Token auth
type AAPClientTokenAuthenticator struct {
	token string // Required
}

// NewTokenAuthenticator creates a new token authentication authenticator
func NewTokenAuthenticator(token *string) (*AAPClientTokenAuthenticator, diag.Diagnostics) {
	var diags diag.Diagnostics
	if token == nil {
		// token must be supplied. If not, that's an error
		diags.AddError(
			"Missing token",
			"Unable to create a token authenticator without token")
	}

	if diags.HasError() {
		return nil, diags
	}

	return &AAPClientTokenAuthenticator{
		token: *token,
	}, nil
}

// Configure configures the HTTP request with token authentication
func (a *AAPClientTokenAuthenticator) Configure(req *http.Request) {
	header := "Authorization"
	prefix := "Bearer"
	req.Header.Set(header, fmt.Sprintf("%s %s", prefix, a.token))
}
