package com

import (
	"net/http"
	"net/url"
)

// scopeTransport is an http.RoundTripper that resolves request URLs against a
// base URL and identifies the client with a fixed User-Agent.
type scopeTransport struct {
	transport http.RoundTripper
	baseUrl   *url.URL
	userAgent string
}

// NewScopeTransport returns an http.RoundTripper that resolves every request URL
// against baseUrl and sets the User-Agent header to userAgent before handing the
// request to transport. It panics if transport or baseUrl is nil.
func NewScopeTransport(transport http.RoundTripper, baseUrl *url.URL, userAgent string) http.RoundTripper {
	if transport == nil {
		panic("NewScopeTransport requires a non-nil transport")
	}

	return &scopeTransport{
		transport: transport,
		baseUrl:   baseUrl,
		userAgent: userAgent,
	}
}

// RoundTrip executes a single HTTP transaction against the scoped base URL.
func (t *scopeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL = t.baseUrl.ResolveReference(req.URL)
	req.Header.Add("User-Agent", t.userAgent)

	return t.transport.RoundTrip(req)
}
