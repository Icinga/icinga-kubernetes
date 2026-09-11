package com

import (
	"net/http"
)

// basicAuthTransport is an http.RoundTripper that authenticates all requests
// using HTTP Basic Authentication.
type basicAuthTransport struct {
	transport http.RoundTripper
	username  string
	password  string
}

// NewBasicAuthTransport returns an http.RoundTripper that adds basic auth credentials
// to every request before handing it to transport. It panics if transport is nil.
func NewBasicAuthTransport(transport http.RoundTripper, username, password string) http.RoundTripper {
	if transport == nil {
		panic("NewBasicAuthTransport requires a non-nil transport")
	}

	return &basicAuthTransport{
		transport: transport,
		username:  username,
		password:  password,
	}
}

// RoundTrip executes a single HTTP transaction with the basic auth credentials.
func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTrip must not modify the caller's request.
	req = req.Clone(req.Context())
	req.SetBasicAuth(t.username, t.password)

	return t.transport.RoundTrip(req)
}
