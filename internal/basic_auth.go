package internal

import (
	"encoding/base64"
	"net/http"
)

// BasicAuthTransport is a http.RoundTripper that authenticates all requests using HTTP Basic Authentication.
type BasicAuthTransport struct {
	Username string
	Password string
}

// RoundTrip executes a single HTTP transaction with the basic auth credentials.
func (rt *BasicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(rt.Username+":"+rt.Password))
	req.Header.Set("Authorization", basicAuth)

	return http.DefaultTransport.RoundTrip(req)
}
