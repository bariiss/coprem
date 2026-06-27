package github

import (
	"net/http"
	"testing"
)

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://api.github.com", false},
		{"http://localhost:8080", false},
		{"https://gh.enterprise.example.com/api/v3", false},
		{"file:///etc/passwd", true},
		{"gopher://evil.example.com", true},
		{"ftp://example.com", true},
		{"", true}, // no host
		{"https://", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := validateBaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBaseURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestNewClientDefaultsTimeout(t *testing.T) {
	c := NewClient(ClientOptions{Token: "x"})
	if c.httpClient.Timeout == 0 {
		t.Error("expected non-zero default timeout when no *http.Client provided")
	}
}

func TestNewClientPreservesCustomHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := NewClient(ClientOptions{HTTPClient: custom, Token: "x"})
	if c.httpClient != custom {
		t.Error("expected client to preserve the caller-provided *http.Client")
	}
}
