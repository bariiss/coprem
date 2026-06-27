package cmd

import "testing"

func TestValidateEnvName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"COPREM_TOKEN", false},
		{"GITHUB_TOKEN", false},
		{"_PRIVATE", false},
		{"A", false},
		{"ABC123", false},
		{"", true},                  // empty
		{"1ABC", true},              // starts with digit
		{"FOO-BAR", true},           // hyphen
		{"FOO$(id)", true},          // command substitution
		{"FOO;rm -rf /", true},      // command injection
		{"FOO BAR", true},           // space
		{"FOO=1", true},             // equals
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnvName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnvName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
