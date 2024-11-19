package internal

import (
	"net/http"
)

// BasicAuthTransport is a http.RoundTripper that authenticates all requests using HTTP Basic Authentication.
type BasicAuthTransport struct {
	Username string
	Password string
}

// RoundTrip executes a single HTTP transaction with the basic auth credentials.
func (rt *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(rt.Username, rt.Password)

	return http.DefaultTransport.RoundTrip(req)
}
