package com

import (
	"crypto/tls"
	"net/http"
)

// BasicAuthTransport is a http.RoundTripper that authenticates all requests using HTTP Basic Authentication.
type BasicAuthTransport struct {
	http.RoundTripper
	Username string
	Password string
	Insecure bool
}

// RoundTrip executes a single HTTP transaction with the basic auth credentials.
func (t *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Username != "" {
		req.SetBasicAuth(t.Username, t.Password)
	}

	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	if t.Insecure {
		if transport, ok := rt.(*http.Transport); ok {
			transportCopy := transport.Clone()
			// #nosec G402 -- TLS certificate verification is intentionally configurable via YAML config.
			transportCopy.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			rt = transportCopy
		}
	}

	return rt.RoundTrip(req)
}
