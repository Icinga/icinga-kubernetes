package com

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newTLSServer starts a TLS test server with a self-signed certificate.
// connState may be nil. It must be set before the server starts, because the
// serving goroutine reads it from then on.
func newTLSServer(t *testing.T, connState func(net.Conn, http.ConnState)) *httptest.Server {
	t.Helper()

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	server.Config.ConnState = connState

	server.StartTLS()
	t.Cleanup(server.Close)

	return server
}

// newCountingServer starts a TLS test server that counts the connections opened against it.
func newCountingServer(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var conns atomic.Int64

	server := newTLSServer(t, func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	})

	return server, &conns
}

// doRequest performs a request and drains the body, which is what allows the connection to be reused.
func doRequest(t *testing.T, client *http.Client, url string) error {
	t.Helper()

	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	_, err = io.Copy(io.Discard, resp.Body)

	return err
}

// TestNewTransportReusesConnections guards against configuring TLS per request instead of once.
func TestNewTransportReusesConnections(t *testing.T) {
	const requests = 10

	tests := []struct {
		name      string
		transport func(http.RoundTripper) http.RoundTripper
	}{
		{
			name: "plain",
			transport: func(rt http.RoundTripper) http.RoundTripper {
				return rt
			},
		},
		{
			name: "wrapped in basicAuthTransport",
			transport: func(rt http.RoundTripper) http.RoundTripper {
				return NewBasicAuthTransport(rt, "user", "pass")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, conns := newCountingServer(t)
			client := &http.Client{Transport: tc.transport(NewTransport(true))}

			for i := range requests {
				if err := doRequest(t, client, server.URL); err != nil {
					t.Fatalf("request %d: %v", i, err)
				}
			}

			if got := conns.Load(); got != 1 {
				t.Errorf("got %d connections for %d requests, want 1", got, requests)
			}
		})
	}
}

func TestNewTransportInsecureSkipsCertificateVerification(t *testing.T) {
	server := newTLSServer(t, nil)
	client := &http.Client{Transport: NewTransport(true)}

	if err := doRequest(t, client, server.URL); err != nil {
		t.Errorf("got error %v, want the self-signed certificate to be accepted", err)
	}
}

func TestNewTransportVerifiesCertificateByDefault(t *testing.T) {
	server := newTLSServer(t, nil)
	client := &http.Client{Transport: NewTransport(false)}

	if err := doRequest(t, client, server.URL); err == nil {
		t.Error("got no error, want the self-signed certificate to be rejected")
	}
}
