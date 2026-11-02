package support

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"conduit/backend/internal/shared/infrastructure/mysql"
)

const DefaultTestDatabaseURL = "mysql://dev_user:dev_pass@localhost:3306/conduit_dev"

type TestDatabase struct {
	Name string
	URL  string
}

func StartTestDatabase(tb testing.TB) *TestDatabase {
	tb.Helper()

	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		adminURL = DefaultTestDatabaseURL
	}
	name := RandomDatabaseName()

	withAdminConn(tb, adminURL, func(ctx context.Context, db *sql.DB) error {
		_, err := db.ExecContext(ctx, "CREATE DATABASE "+quoteIdentifier(name)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")
		return err
	})
	tb.Cleanup(func() {
		withAdminConn(tb, adminURL, func(ctx context.Context, db *sql.DB) error {
			_, err := db.ExecContext(ctx, "DROP DATABASE IF EXISTS "+quoteIdentifier(name))
			return err
		})
	})

	parsed, err := url.Parse(adminURL)
	if err != nil {
		tb.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}
	parsed.Path = "/" + name
	return &TestDatabase{Name: name, URL: parsed.String()}
}

func RandomDatabaseName() string {
	suffix := make([]byte, 5)
	if _, err := rand.Read(suffix); err != nil {
		panic(err)
	}
	return "conduit_test_" + hex.EncodeToString(suffix)
}

func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func withAdminConn(tb testing.TB, adminURL string, run func(context.Context, *sql.DB) error) {
	tb.Helper()
	ctx := context.Background()
	db, err := mysql.Connect(ctx, mysql.ConnectionConfig{URL: adminURL, PoolMax: 1, Timeout: 10 * time.Second})
	if err != nil {
		tb.Fatalf("cannot connect to the test MySQL server: %v", err)
	}
	defer db.Close()
	if err := run(ctx, db); err != nil {
		tb.Fatalf("admin statement failed: %v", err)
	}
}
