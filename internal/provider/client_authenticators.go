package provider

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type AAPClientAuthenticator interface {
	Configure(*http.Request)
}

// Basic authenticator supports username/password auth
type AAPClientBasicAuthenticator struct {
	username string
	password string
}

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

func (a *AAPClientBasicAuthenticator) Configure(req *http.Request) {
	// To configure basic auth, we can just use http.Request's SetBasicAuth
	req.SetBasicAuth(a.username, a.password)
}

// Token authenticator supports Token auth
type AAPClientTokenAuthenticator struct {
	token string // Required
}

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

func (a *AAPClientTokenAuthenticator) Configure(req *http.Request) {
	header := "Authorization"
	prefix := "Bearer"
	req.Header.Set(header, fmt.Sprintf("%s %s", prefix, a.token))
}
