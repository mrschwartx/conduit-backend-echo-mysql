package infrastructure

import (
	"context"

	"conduit/backend/internal/shared/infrastructure/mysql"
)

type MySQLFollowRepository struct {
	db mysql.Querier
}

func NewMySQLFollowRepository(db mysql.Querier) *MySQLFollowRepository {
	return &MySQLFollowRepository{db: db}
}

func (r *MySQLFollowRepository) Follow(ctx context.Context, followerID, followedID string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO follows (follower_id, followed_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE follower_id = follower_id",
		followerID, followedID,
	)
	return err
}

func (r *MySQLFollowRepository) Unfollow(ctx context.Context, followerID, followedID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM follows WHERE follower_id = ? AND followed_id = ?",
		followerID, followedID,
	)
	return err
}

func (r *MySQLFollowRepository) IsFollowing(ctx context.Context, followerID, followedID string) (bool, error) {
	var following bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = ? AND followed_id = ?)",
		followerID, followedID,
	).Scan(&following)
	return following, err
}

func (r *MySQLFollowRepository) FindFollowedAmong(ctx context.Context, followerID string, candidateIDs []string) (map[string]bool, error) {
	followed := map[string]bool{}
	if len(candidateIDs) == 0 {
		return followed, nil
	}
	args := append([]any{followerID}, mysql.StringArgs(candidateIDs)...)
	rows, err := r.db.QueryContext(ctx,
		"SELECT followed_id FROM follows WHERE follower_id = ? AND followed_id IN ("+mysql.Placeholders(len(candidateIDs))+")",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		followed[id] = true
	}
	return followed, rows.Err()
}

func (r *MySQLFollowRepository) ListFollowedIDs(ctx context.Context, followerID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT followed_id FROM follows WHERE follower_id = ?",
		followerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
