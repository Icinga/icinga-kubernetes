package com

import (
	"net/http"
)

// BasicAuthTransport is a http.RoundTripper that authenticates all requests using HTTP Basic Authentication.
type BasicAuthTransport struct {
	http.RoundTripper
	Username string
	Password string
}

// RoundTrip executes a single HTTP transaction with the basic auth credentials.
func (t *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Username != "" {
		// RoundTrip must not modify the caller's request.
		req = req.Clone(req.Context())
		req.SetBasicAuth(t.Username, t.Password)
	}

	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	return rt.RoundTrip(req)
}
