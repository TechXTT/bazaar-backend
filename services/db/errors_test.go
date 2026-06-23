package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestIsUniqueViolation covers BE-12: only a Postgres 23505 error counts as a
// unique-constraint violation.
func TestIsUniqueViolation(t *testing.T) {
	uniqueErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
	otherPgErr := &pgconn.PgError{Code: "23503", Message: "foreign key violation"}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"unique violation", uniqueErr, true},
		{"wrapped unique violation", fmt.Errorf("create: %w", uniqueErr), true},
		{"other pg error", otherPgErr, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUniqueViolation(tc.err); got != tc.want {
				t.Fatalf("IsUniqueViolation = %v, want %v", got, tc.want)
			}
		})
	}
}
