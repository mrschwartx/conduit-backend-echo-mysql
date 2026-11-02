package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

const (
	urlScheme       = "mysql://"
	defaultPort     = "3306"
	connMaxLifetime = 30 * time.Minute
	isolationParam  = "transaction_isolation"
	isolationLevel  = "'READ-COMMITTED'"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type DB interface {
	Querier
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type ConnectionConfig struct {
	URL     string
	PoolMax int
	Timeout time.Duration
}

func Connect(ctx context.Context, cfg ConnectionConfig) (*sql.DB, error) {
	driverConfig, err := ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	if cfg.Timeout > 0 {
		driverConfig.Timeout = cfg.Timeout
	}

	connector, err := driver.NewConnector(driverConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	db := sql.OpenDB(connector)
	if cfg.PoolMax > 0 {
		db.SetMaxOpenConns(cfg.PoolMax)
		db.SetMaxIdleConns(cfg.PoolMax)
	}
	db.SetConnMaxLifetime(connMaxLifetime)

	pingCtx := ctx
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		pingCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database connection failed: %w", err)
	}
	return db, nil
}

func ParseURL(raw string) (*driver.Config, error) {
	dsn := raw
	if strings.HasPrefix(raw, urlScheme) {
		converted, err := urlToDSN(raw)
		if err != nil {
			return nil, err
		}
		dsn = converted
	}

	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.ClientFoundRows = true
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, ok := cfg.Params[isolationParam]; !ok {
		cfg.Params[isolationParam] = isolationLevel
	}
	return cfg, nil
}

func urlToDSN(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("missing host")
	}

	port := parsed.Port()
	if port == "" {
		port = defaultPort
	}

	var dsn strings.Builder
	if parsed.User != nil {
		dsn.WriteString(parsed.User.Username())
		if password, ok := parsed.User.Password(); ok {
			dsn.WriteString(":" + password)
		}
		dsn.WriteString("@")
	}
	dsn.WriteString("tcp(" + net.JoinHostPort(parsed.Hostname(), port) + ")/")
	dsn.WriteString(strings.TrimPrefix(parsed.Path, "/"))
	if parsed.RawQuery != "" {
		dsn.WriteString("?" + parsed.RawQuery)
	}
	return dsn.String(), nil
}
