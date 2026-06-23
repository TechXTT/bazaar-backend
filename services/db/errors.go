package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolationCode is the Postgres SQLSTATE for a unique-constraint
// violation.
const uniqueViolationCode = "23505"

// IsUniqueViolation reports whether err is a Postgres unique-constraint
// violation. It lets services rely on a DB-enforced unique index (BE-12)
// instead of a racy app-level "does this name already exist?" pre-check.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == uniqueViolationCode
	}
	return false
}
