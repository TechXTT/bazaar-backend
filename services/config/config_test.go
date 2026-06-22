package config

import "testing"

// TestNewConfig_CORSWildcardWithCredentials verifies the BE-1 fail-fast guard:
// "*" origins or headers combined with credentials must be rejected at load.
func TestNewConfig_CORSWildcardWithCredentials(t *testing.T) {
	cases := []struct {
		name        string
		origins     string
		headers     string
		credentials string
		wantErr     bool
	}{
		{
			name:        "wildcard origin with credentials is rejected",
			origins:     "http://localhost:3000,*",
			headers:     "Content-Type,Authorization",
			credentials: "true",
			wantErr:     true,
		},
		{
			name:        "wildcard header with credentials is rejected",
			origins:     "http://localhost:3000",
			headers:     "*",
			credentials: "true",
			wantErr:     true,
		},
		{
			name:        "wildcard with spaces is still caught",
			origins:     "http://localhost:3000, * ",
			headers:     "Content-Type",
			credentials: "true",
			wantErr:     true,
		},
		{
			name:        "explicit origins and headers with credentials is allowed",
			origins:     "http://localhost:3000,http://localhost:8000",
			headers:     "Content-Type,Authorization",
			credentials: "true",
			wantErr:     false,
		},
		{
			name:        "wildcard without credentials is allowed",
			origins:     "*",
			headers:     "*",
			credentials: "false",
			wantErr:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HTTP_ALLOWED_ORIGINS", tc.origins)
			t.Setenv("HTTP_ALLOWED_HEADERS", tc.headers)
			t.Setenv("HTTP_ALLOWED_CREDENTIALS", tc.credentials)

			_, err := NewConfig(nil)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
