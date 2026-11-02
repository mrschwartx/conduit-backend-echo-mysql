package mysql

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"strings"
)

const (
	migrationLockName           = "conduit_schema_migrations"
	migrationLockTimeoutSeconds = 60
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

func EmbeddedMigrations() fs.FS {
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

func RunMigrations(ctx context.Context, db *sql.DB, source fs.FS, logger *slog.Logger) ([]string, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := acquireLock(ctx, conn); err != nil {
		return nil, err
	}
	defer releaseLock(context.WithoutCancel(ctx), conn)

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		  name       VARCHAR(255) CHARACTER SET ascii NOT NULL,
		  applied_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
		  PRIMARY KEY (name)
		) ENGINE=InnoDB`); err != nil {
		return nil, err
	}

	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return nil, err
	}

	files, err := listMigrationFiles(source)
	if err != nil {
		return nil, err
	}

	executed := []string{}
	for _, file := range files {
		if applied[file] {
			continue
		}
		if err := applyMigration(ctx, conn, source, file); err != nil {
			return executed, fmt.Errorf("migration %s failed: %w", file, err)
		}
		executed = append(executed, file)
		if logger != nil {
			logger.Info("Applied migration " + file)
		}
	}
	return executed, nil
}

func acquireLock(ctx context.Context, conn Querier) error {
	var acquired sql.NullInt64
	err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", migrationLockName, migrationLockTimeoutSeconds).Scan(&acquired)
	if err != nil {
		return err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("could not acquire the migration lock")
	}
	return nil
}

func releaseLock(ctx context.Context, conn Querier) {
	var released sql.NullInt64
	_ = conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", migrationLockName).Scan(&released)
}

func appliedMigrations(ctx context.Context, conn Querier) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func listMigrationFiles(source fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	slices.Sort(files)
	return files, nil
}

func applyMigration(ctx context.Context, conn Querier, source fs.FS, file string) error {
	script, err := fs.ReadFile(source, file)
	if err != nil {
		return err
	}
	for _, statement := range splitStatements(string(script)) {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	_, err = conn.ExecContext(ctx, "INSERT INTO schema_migrations (name) VALUES (?)", file)
	return err
}

func splitStatements(script string) []string {
	statements := []string{}
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if statement := strings.TrimSpace(current.String()); statement != "" {
			statements = append(statements, statement)
		}
		current.Reset()
	}

	for _, character := range script {
		if quote != 0 {
			current.WriteRune(character)
			switch {
			case escaped:
				escaped = false
			case character == '\\' && quote != '`':
				escaped = true
			case character == quote:
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"', '`':
			quote = character
			current.WriteRune(character)
		case ';':
			flush()
		default:
			current.WriteRune(character)
		}
	}
	flush()
	return statements
}
