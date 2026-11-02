package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"

	"conduit/backend/internal/config"
	"conduit/backend/internal/modules/articles"
	"conduit/backend/internal/modules/comments"
	"conduit/backend/internal/modules/users"
	"conduit/backend/internal/shared/infrastructure/events"
	"conduit/backend/internal/shared/infrastructure/mysql"
)

type Container struct {
	Config   config.Config
	Logger   *slog.Logger
	DB       *sql.DB
	Events   *events.InMemoryBus
	Users    *users.Module
	Articles *articles.Module
	Comments *comments.Module
}

func NewContainer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Container, error) {
	db, err := mysql.Connect(ctx, mysql.ConnectionConfig{
		URL:     cfg.Database.URL,
		PoolMax: cfg.Database.PoolMax,
		Timeout: cfg.Database.Timeout,
	})
	if err != nil {
		return nil, err
	}

	if _, err := mysql.RunMigrations(ctx, db, mysql.EmbeddedMigrations(), logger); err != nil {
		_ = db.Close()
		return nil, err
	}

	bus := events.NewInMemoryBus(logger)
	usersModule := users.NewModule(users.Dependencies{DB: db, Config: cfg})
	articlesModule := articles.NewModule(articles.Dependencies{
		DB:                   db,
		Events:               bus,
		Authors:              usersModule.AuthorDirectory,
		Authenticate:         usersModule.Authenticate,
		OptionalAuthenticate: usersModule.OptionalAuthenticate,
	})
	commentsModule := comments.NewModule(comments.Dependencies{
		DB:                   db,
		Events:               bus,
		Authors:              usersModule.AuthorDirectory,
		Articles:             articlesModule.Catalog,
		Authenticate:         usersModule.Authenticate,
		OptionalAuthenticate: usersModule.OptionalAuthenticate,
	})

	return &Container{
		Config:   cfg,
		Logger:   logger,
		DB:       db,
		Events:   bus,
		Users:    usersModule,
		Articles: articlesModule,
		Comments: commentsModule,
	}, nil
}

func (c *Container) Close() {
	_ = c.DB.Close()
}
