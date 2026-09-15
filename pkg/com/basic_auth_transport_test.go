package com

import (
	"net/http"
	"testing"
)

const (
	testUsername = "user"
	testPassword = "s3cret"
)

// recordingTransport captures the request it is handed and answers it without
// a network round trip.
type recordingTransport struct {
	req *http.Request
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.req = req

	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
}

func newRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "https://example.com/api/v1/query", nil)
	if err != nil {
		t.Fatalf("cannot create request: %v", err)
	}

	return req
}

func TestBasicAuthTransportSetsCredentials(t *testing.T) {
	inner := &recordingTransport{}
	req := newRequest(t)

	if _, err := NewBasicAuthTransport(inner, testUsername, testPassword).RoundTrip(req); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	gotUsername, gotPassword, ok := inner.req.BasicAuth()
	if !ok {
		t.Fatal("got a request without basic auth credentials, want them set")
	}

	if gotUsername != testUsername || gotPassword != testPassword {
		t.Errorf("got credentials %q:%q, want %q:%q", gotUsername, gotPassword, testUsername, testPassword)
	}
}

// TestBasicAuthTransportDoesNotModifyRequest guards against credentials leaking
// into a request the caller may reuse or inspect afterward.
func TestBasicAuthTransportDoesNotModifyRequest(t *testing.T) {
	inner := &recordingTransport{}
	req := newRequest(t)

	if _, err := NewBasicAuthTransport(inner, testUsername, testPassword).RoundTrip(req); err != nil {
		t.Fatalf("got error %v, want none", err)
	}

	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("got Authorization %q on the caller's request, want it untouched", got)
	}
}

func TestNewBasicAuthTransportRejectsNilTransport(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("got no panic, want a panic for a nil transport")
		}
	}()

	NewBasicAuthTransport(nil, testUsername, testPassword)
}
