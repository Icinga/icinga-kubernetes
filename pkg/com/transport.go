package com

import (
	"crypto/tls"
	"net/http"
)

// NewTransport returns an *http.Transport based on http.DefaultTransport.
// TLS certificate verification is skipped if insecure is true.
func NewTransport(insecure bool) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if insecure {
		// #nosec G402 -- TLS certificate verification is intentionally configurable via YAML config.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return transport
}
