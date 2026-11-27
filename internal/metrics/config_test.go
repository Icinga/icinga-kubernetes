package metrics

import "testing"

func TestPrometheusConfigValidate(t *testing.T) {
	const url = "http://prometheus:9090"

	tests := map[string]struct {
		config  PrometheusConfig
		wantErr bool
	}{
		"Empty":                {PrometheusConfig{}, false},
		"Url":                  {PrometheusConfig{Url: url}, false},
		"BasicAuth":            {PrometheusConfig{Url: url, Username: "u", Password: "p"}, false},
		"Token":                {PrometheusConfig{Url: url, Token: "t"}, false},
		"TokenFile":            {PrometheusConfig{Url: url, TokenFile: "/f"}, false},
		"UsernameOnly":         {PrometheusConfig{Url: url, Username: "u"}, true},
		"BasicAuthAndToken":    {PrometheusConfig{Url: url, Username: "u", Password: "p", Token: "t"}, true},
		"BasicAuthAndFile":     {PrometheusConfig{Url: url, Username: "u", Password: "p", TokenFile: "/f"}, true},
		"TokenAndFile":         {PrometheusConfig{Url: url, Token: "t", TokenFile: "/f"}, true},
		"InvalidInsecure":      {PrometheusConfig{Url: url, Insecure: "yes"}, true},
		"NoUrlUsernameOnly":    {PrometheusConfig{Username: "u"}, false},
		"NoUrlBasicAuthToken":  {PrometheusConfig{Username: "u", Password: "p", Token: "t"}, true},
		"NoUrlBasicAuthFile":   {PrometheusConfig{Username: "u", Password: "p", TokenFile: "/f"}, true},
		"NoUrlTokenAndFile":    {PrometheusConfig{Token: "t", TokenFile: "/f"}, true},
		"NoUrlUsernameAndFile": {PrometheusConfig{Username: "u", TokenFile: "/f"}, true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
