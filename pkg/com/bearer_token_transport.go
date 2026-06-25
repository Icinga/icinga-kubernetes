package com

import (
	"net/http"
	"os"
	"strings"
)

// BearerTokenTransport is a http.RoundTripper that authenticates all requests using a bearer token.
type BearerTokenTransport struct {
	http.RoundTripper
	Token     string
	TokenFile string
}

// RoundTrip executes a single HTTP transaction with the bearer token.
func (t *BearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := t.Token
	if token == "" && t.TokenFile != "" {
		contents, err := os.ReadFile(t.TokenFile)
		if err != nil {
			return nil, err
		}

		token = strings.TrimSpace(string(contents))
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rt := t.RoundTripper
	if rt == nil {
		rt = http.DefaultTransport
	}

	return rt.RoundTrip(req)
}
