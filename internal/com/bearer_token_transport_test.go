package com

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testToken = "s3cret-token"

// writeTokenFile creates a token file and returns its path.
func writeTokenFile(t *testing.T, token string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatalf("cannot write token file: %v", err)
	}

	return path
}

func TestBearerTokenTransportSetsAuthorization(t *testing.T) {
	inner := &recordingTransport{}
	req := newRequest(t)

	if _, err := NewBearerTokenTransport(inner, testToken).RoundTrip(req); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	if got, want := inner.req.Header.Get("Authorization"), "Bearer "+testToken; got != want {
		t.Errorf("got Authorization %q, want %q", got, want)
	}
}

// TestBearerTokenTransportDoesNotModifyRequest guards against the token leaking
// into a request the caller may reuse or inspect afterward.
func TestBearerTokenTransportDoesNotModifyRequest(t *testing.T) {
	inner := &recordingTransport{}
	req := newRequest(t)

	if _, err := NewBearerTokenTransport(inner, testToken).RoundTrip(req); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("got Authorization %q on the caller's request, want it untouched", got)
	}
}

func TestNewBearerTokenTransportRejectsNilTransport(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("got no panic, want a panic for a nil transport")
		}
	}()

	NewBearerTokenTransport(nil, testToken)
}

// TestBearerTokenFileTransportTrimsToken covers the trailing newline that
// Kubernetes service account token files and shell redirections leave behind.
func TestBearerTokenFileTransportTrimsToken(t *testing.T) {
	inner := &recordingTransport{}
	tokenFile := writeTokenFile(t, "  "+testToken+"\n")

	if _, err := NewBearerTokenFileTransport(inner, tokenFile).RoundTrip(newRequest(t)); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	if got, want := inner.req.Header.Get("Authorization"), "Bearer "+testToken; got != want {
		t.Errorf("got Authorization %q, want %q", got, want)
	}
}

// TestBearerTokenFileTransportPicksUpRotatedToken guards against caching the
// token, which would break once Kubernetes rotates it.
func TestBearerTokenFileTransportPicksUpRotatedToken(t *testing.T) {
	inner := &recordingTransport{}
	tokenFile := writeTokenFile(t, testToken)
	transport := NewBearerTokenFileTransport(inner, tokenFile)

	if _, err := transport.RoundTrip(newRequest(t)); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	const rotated = "rotated-token"
	if err := os.WriteFile(tokenFile, []byte(rotated), 0o600); err != nil {
		t.Fatalf("cannot rotate token file: %v", err)
	}

	if _, err := transport.RoundTrip(newRequest(t)); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	if got, want := inner.req.Header.Get("Authorization"), "Bearer "+rotated; got != want {
		t.Errorf("got Authorization %q, want %q", got, want)
	}
}

// TestBearerTokenFileTransportFailsOnUnreadableFile guards against sending an
// unauthenticated request when the token cannot be read.
func TestBearerTokenFileTransportFailsOnUnreadableFile(t *testing.T) {
	inner := &recordingTransport{}
	tokenFile := filepath.Join(t.TempDir(), "missing")

	if _, err := NewBearerTokenFileTransport(inner, tokenFile).RoundTrip(newRequest(t)); err == nil {
		t.Error("got no error, want the missing token file to be reported")
	}

	if inner.req != nil {
		t.Error("got a request handed to the inner transport, want none")
	}
}

// TestBearerTokenFileTransportFailsOnEmptyFile guards against sending an empty
// bearer token, which Prometheus would only reject with an opaque 401.
func TestBearerTokenFileTransportFailsOnEmptyFile(t *testing.T) {
	inner := &recordingTransport{}
	tokenFile := writeTokenFile(t, " \n")

	if _, err := NewBearerTokenFileTransport(inner, tokenFile).RoundTrip(newRequest(t)); err == nil {
		t.Error("got no error, want the empty token file to be reported")
	}

	if inner.req != nil {
		t.Error("got a request handed to the inner transport, want none")
	}
}

func TestNewBearerTokenFileTransportRejectsNilTransport(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("got no panic, want a panic for a nil transport")
		}
	}()

	NewBearerTokenFileTransport(nil, "/dev/null")
}

// closeTrackingBody records whether the request body was closed.
type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true

	return nil
}

// TestBearerTokenFileTransportClosesBodyOnError covers the http.RoundTripper
// contract, which requires closing the body even when the round trip fails.
func TestBearerTokenFileTransportClosesBodyOnError(t *testing.T) {
	body := &closeTrackingBody{Reader: strings.NewReader("query=up")}

	req, err := http.NewRequest(http.MethodPost, "https://example.com/api/v1/query", body)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	tokenFile := filepath.Join(t.TempDir(), "missing")
	if _, err := NewBearerTokenFileTransport(&recordingTransport{}, tokenFile).RoundTrip(req); err == nil {
		t.Fatal("got no error, want the missing token file to be reported")
	}

	if !body.closed {
		t.Error("got an unclosed request body, want it closed")
	}
}
