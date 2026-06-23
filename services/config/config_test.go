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
			// Satisfy BE-20's required-DB-config validation so the CORS cases
			// reach their intended assertion rather than failing on missing DB.
			t.Setenv("POSTGRES_HOST", "localhost")
			t.Setenv("POSTGRES_USER", "postgres")
			t.Setenv("POSTGRES_DB", "bazaar")

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

// TestNewConfig_Validation covers BE-20: invalid numeric/duration values and
// missing required DB config must fail fast at load instead of silently
// producing a misconfigured process.
func TestNewConfig_Validation(t *testing.T) {
	setValidDB := func(t *testing.T) {
		t.Setenv("POSTGRES_HOST", "localhost")
		t.Setenv("POSTGRES_USER", "postgres")
		t.Setenv("POSTGRES_DB", "bazaar")
	}

	t.Run("unparseable HTTP_PORT fails", func(t *testing.T) {
		setValidDB(t)
		t.Setenv("HTTP_PORT", "not-a-number")
		if _, err := NewConfig(nil); err == nil {
			t.Fatal("expected error for invalid HTTP_PORT")
		}
	})

	t.Run("out-of-range HTTP_PORT fails", func(t *testing.T) {
		setValidDB(t)
		t.Setenv("HTTP_PORT", "70000")
		if _, err := NewConfig(nil); err == nil {
			t.Fatal("expected error for out-of-range HTTP_PORT")
		}
	})

	t.Run("bad HTTP_READ_TIMEOUT fails", func(t *testing.T) {
		setValidDB(t)
		t.Setenv("HTTP_READ_TIMEOUT", "fifteen-seconds")
		if _, err := NewConfig(nil); err == nil {
			t.Fatal("expected error for invalid HTTP_READ_TIMEOUT")
		}
	})

	t.Run("missing POSTGRES_HOST fails", func(t *testing.T) {
		t.Setenv("POSTGRES_HOST", "")
		t.Setenv("POSTGRES_USER", "postgres")
		t.Setenv("POSTGRES_DB", "bazaar")
		if _, err := NewConfig(nil); err == nil {
			t.Fatal("expected error for missing POSTGRES_HOST")
		}
	})

	t.Run("valid config with defaults loads", func(t *testing.T) {
		setValidDB(t)
		t.Setenv("HTTP_PORT", "")
		t.Setenv("HTTP_READ_TIMEOUT", "")
		cfg, err := NewConfig(nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.GetHTTP().Port != 8000 {
			t.Fatalf("expected default port 8000, got %d", cfg.GetHTTP().Port)
		}
	})
}
