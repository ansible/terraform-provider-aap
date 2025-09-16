package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestNewBasicAuthenticator(t *testing.T) {
	testUsername := "testusername"
	testPassword := "testpassword"
	var testTable = []struct {
		name          string
		username      *string
		password      *string
		expectSuccess bool
	}{
		{
			name:          "Success when providing username and password",
			username:      &testUsername,
			password:      &testPassword,
			expectSuccess: true,
		},
		{
			name:          "Failure when username and password are nil",
			username:      nil,
			password:      nil,
			expectSuccess: false,
		},
		{
			name:          "Failure when username is provided and password is nil",
			username:      &testUsername,
			password:      nil,
			expectSuccess: false,
		},
		{
			name:          "Failure with when username is nil and password is provided",
			username:      nil,
			password:      &testPassword,
			expectSuccess: false,
		},
	}
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			auth, diags := NewBasicAuthenticator(test.username, test.password)
			if test.expectSuccess {
				if auth == nil {
					t.Errorf("Expected NewBasicAuthenticator result to be defined, failed with %v", diags)
				}
			} else {
				if !diags.HasError() {
					t.Errorf("Expected NewBasicAuthenticator to fail, received %v", auth)
				}
			}
		})
	}
}

func TestBasicAuthenticatorConfigure(t *testing.T) {
	testUsername := "username"
	testPassword := "password"
	auth, _ := NewBasicAuthenticator(&testUsername, &testPassword)
	req, _ := http.NewRequestWithContext(context.TODO(), http.MethodGet, "", strings.NewReader(""))
	auth.Configure(req)
	actual := req.Header["Authorization"][0]
	expected := "Basic dXNlcm5hbWU6cGFzc3dvcmQ=" // base64 encoding of string "username:password"
	if actual != expected {
		t.Errorf("Expected (%s) not equal to actual (%s)", expected, actual)
	}
}

func TestNewTokenAuthenticator(t *testing.T) {
	testToken := "testtoken"
	var testTable = []struct {
		name          string
		token         *string
		expectSuccess bool
	}{
		{
			name:          "Success when token is provided",
			token:         &testToken,
			expectSuccess: true,
		},
		{
			name:          "Failure when token is nil ",
			token:         nil,
			expectSuccess: false,
		},
	}
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			auth, diags := NewTokenAuthenticator(test.token)
			if test.expectSuccess {
				if auth == nil {
					t.Errorf("Expected NewTokenAuthenticator result to be defined, failed with %v", diags)
				}
			} else {
				if !diags.HasError() {
					t.Errorf("Expected NewTokenAuthenticator to fail, received %v", auth)
				}
			}
		})
	}
}

func TestTokenAuthenticatorConfigure(t *testing.T) {
	testToken := "testtoken"

	var testTable = []struct {
		name         string
		token        *string
		expectHeader string
		expectValue  string
	}{
		{
			name:         "Configure defaults header to Authorization: Bearer ...",
			token:        &testToken,
			expectHeader: "Authorization",
			expectValue:  "Bearer testtoken",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			auth, _ := NewTokenAuthenticator(test.token)
			req, _ := http.NewRequestWithContext(context.TODO(), http.MethodGet, "", strings.NewReader(""))
			auth.Configure(req)
			actual := req.Header[test.expectHeader][0]
			expected := test.expectValue
			if actual != expected {
				t.Errorf("Expected (%s) not equal to actual (%s)", expected, actual)
			}
		})
	}
}
