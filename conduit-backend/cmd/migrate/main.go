package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"conduit/backend/internal/config"
	"conduit/backend/internal/shared/infrastructure/logging"
	"conduit/backend/internal/shared/infrastructure/mysql"
)

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()
	ctx := context.Background()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed: "+err.Error())
		return 1
	}
	logger, closeLogger, err := logging.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed: "+err.Error())
		return 1
	}
	defer closeLogger()

	db, err := mysql.Connect(ctx, mysql.ConnectionConfig{
		URL:     cfg.Database.URL,
		PoolMax: cfg.Database.PoolMax,
		Timeout: cfg.Database.Timeout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed: "+err.Error())
		return 1
	}
	defer db.Close()

	executed, err := mysql.RunMigrations(ctx, db, mysql.EmbeddedMigrations(), logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed: "+err.Error())
		return 1
	}
	logger.Info(fmt.Sprintf("Migrations completed, %d applied", len(executed)))
	return 0
}
