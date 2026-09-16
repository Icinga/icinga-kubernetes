package com

import (
	"net/http"
	"net/url"
)

// baseUrlTransport is an http.RoundTripper that resolves request URLs against a base URL.
type baseUrlTransport struct {
	transport http.RoundTripper
	baseUrl   *url.URL
}

// NewBaseUrlTransport returns an http.RoundTripper that resolves every request
// URL against baseUrl. It panics if transport or baseUrl is nil.
func NewBaseUrlTransport(transport http.RoundTripper, baseUrl *url.URL) http.RoundTripper {
	if transport == nil {
		panic("NewBaseUrlTransport requires a non-nil transport")
	}

	if baseUrl == nil {
		panic("NewBaseUrlTransport requires a non-nil baseUrl")
	}

	return &baseUrlTransport{
		transport: transport,
		baseUrl:   baseUrl,
	}
}

// RoundTrip executes a single HTTP transaction against the base URL.
func (t *baseUrlTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTrip must not modify the caller's request.
	req = req.Clone(req.Context())
	req.URL = t.baseUrl.ResolveReference(req.URL)

	return t.transport.RoundTrip(req)
}
