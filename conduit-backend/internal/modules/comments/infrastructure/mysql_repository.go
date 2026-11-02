package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"conduit/backend/internal/modules/comments/domain"
	"conduit/backend/internal/shared/infrastructure/mysql"
)

const commentColumns = "id, article_id, author_id, body, created_at, updated_at"

type MySQLRepository struct {
	db mysql.Querier
}

func NewMySQLRepository(db mysql.Querier) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Add(ctx context.Context, comment domain.Comment) (domain.Comment, error) {
	snapshot := comment.Snapshot()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO comments (article_id, author_id, body, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		snapshot.ArticleID, snapshot.AuthorID, snapshot.Body, snapshot.CreatedAt, snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.Comment{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Comment{}, err
	}
	return scanComment(r.db.QueryRowContext(ctx, "SELECT "+commentColumns+" FROM comments WHERE id = ?", id))
}

func (r *MySQLRepository) FindByID(ctx context.Context, id int64) (domain.Comment, bool, error) {
	comment, err := scanComment(r.db.QueryRowContext(ctx, "SELECT "+commentColumns+" FROM comments WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Comment{}, false, nil
	}
	if err != nil {
		return domain.Comment{}, false, err
	}
	return comment, true, nil
}

func (r *MySQLRepository) ListByArticle(ctx context.Context, articleID string) ([]domain.Comment, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+commentColumns+" FROM comments WHERE article_id = ? ORDER BY created_at DESC, id DESC",
		articleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []domain.Comment{}
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (r *MySQLRepository) DeleteByID(ctx context.Context, id int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *MySQLRepository) DeleteByArticle(ctx context.Context, articleID string) (int64, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM comments WHERE article_id = ?", articleID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanComment(row rowScanner) (domain.Comment, error) {
	var snapshot domain.Snapshot
	err := row.Scan(
		&snapshot.ID, &snapshot.ArticleID, &snapshot.AuthorID, &snapshot.Body,
		&snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.Comment{}, err
	}
	return domain.Restore(snapshot), nil
}
