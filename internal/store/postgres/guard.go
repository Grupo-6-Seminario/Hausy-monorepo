package postgres

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CheckDisposableURI verifies that the provided PostgreSQL connection URI targets
// a disposable test database (e.g. named "test" or ending in "_test").
// It prevents destructive operations like TRUNCATE / DeleteAll from running against
// development or production databases by mistake.
func CheckDisposableURI(uri string) error {
	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("test database URI is empty")
	}

	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return fmt.Errorf("invalid database URI: %w", err)
	}

	dbName := config.ConnConfig.Database
	if dbName == "" {
		return fmt.Errorf("database URI %q does not specify a database name", uri)
	}

	if strings.HasSuffix(dbName, "_test") || dbName == "test" {
		return nil
	}

	return fmt.Errorf("refusing non-disposable database %q: name must end with '_test' (e.g. hausy_test)", dbName)
}
