package com

import (
	"net/http"
)

// userAgentTransport is an http.RoundTripper that identifies the client with a fixed User-Agent.
type userAgentTransport struct {
	transport http.RoundTripper
	userAgent string
}

// NewUserAgentTransport returns an http.RoundTripper that sets the User-Agent
// header to userAgent before handing the request to transport. It panics if
// transport is nil.
func NewUserAgentTransport(transport http.RoundTripper, userAgent string) http.RoundTripper {
	if transport == nil {
		panic("NewUserAgentTransport requires a non-nil transport")
	}

	return &userAgentTransport{
		transport: transport,
		userAgent: userAgent,
	}
}

// RoundTrip executes a single HTTP transaction with the configured User-Agent.
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTrip must not modify the caller's request.
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.userAgent)

	return t.transport.RoundTrip(req)
}
