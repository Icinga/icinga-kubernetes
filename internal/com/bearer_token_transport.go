package com

import (
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"
)

// bearerTokenTransport is an http.RoundTripper that authenticates all requests
// using a fixed bearer token.
type bearerTokenTransport struct {
	transport http.RoundTripper
	token     string
}

// NewBearerTokenTransport returns an http.RoundTripper that authorizes every
// request with token before handing it to transport. It panics if transport is nil.
func NewBearerTokenTransport(transport http.RoundTripper, token string) http.RoundTripper {
	if transport == nil {
		panic("NewBearerTokenTransport requires a non-nil transport")
	}

	return &bearerTokenTransport{
		transport: transport,
		token:     token,
	}
}

// RoundTrip executes a single HTTP transaction with the bearer token.
func (t *bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTrip must not modify the caller's request.
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)

	return t.transport.RoundTrip(req)
}

// bearerTokenFileTransport is an http.RoundTripper that authenticates all
// requests using a bearer token read from a file.
type bearerTokenFileTransport struct {
	transport http.RoundTripper
	tokenFile string
}

// NewBearerTokenFileTransport returns an http.RoundTripper that authorizes
// every request with the token stored in tokenFile before handing it to
// transport. It panics if transport is nil.
func NewBearerTokenFileTransport(transport http.RoundTripper, tokenFile string) http.RoundTripper {
	if transport == nil {
		panic("NewBearerTokenFileTransport requires a non-nil transport")
	}

	return &bearerTokenFileTransport{
		transport: transport,
		tokenFile: tokenFile,
	}
}

// RoundTrip executes a single HTTP transaction with the bearer token from the token file.
func (t *bearerTokenFileTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// The file is read per request because Kubernetes rotates projected service
	// account tokens in place.
	data, err := os.ReadFile(t.tokenFile)
	token := strings.TrimSpace(string(data))
	if err == nil && token == "" {
		// Sending an empty token would only surface as an opaque 401 from Prometheus.
		err = errors.New("file is empty")
	}
	if err != nil {
		// RoundTrip must close the body even when it fails.
		if req.Body != nil {
			_ = req.Body.Close()
		}

		return nil, errors.Wrapf(err, "cannot read bearer token file %q", t.tokenFile)
	}

	// RoundTrip must not modify the caller's request.
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)

	return t.transport.RoundTrip(req)
}
