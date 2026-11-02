package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/mysql"
)

const slugConstraint = "articles_slug_unique"

const viewColumns = `
  a.id, a.author_id, a.slug, a.title, a.description, a.created_at, a.updated_at,
  (SELECT COUNT(*) FROM favorites f WHERE f.article_id = a.id) AS favorites_count,
  EXISTS (
    SELECT 1 FROM favorites f
     WHERE f.article_id = a.id AND f.user_id = ?
  ) AS favorited`

type MySQLRepository struct {
	db mysql.DB
}

func NewMySQLRepository(db mysql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *MySQLRepository) Add(ctx context.Context, article domain.Article) error {
	snapshot := article.Snapshot()
	err := mysql.WithTransaction(ctx, r.db, func(tx mysql.Querier) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO articles (id, author_id, slug, title, description, body, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshot.ID, snapshot.AuthorID, snapshot.Slug, snapshot.Title, snapshot.Description,
			snapshot.Body, snapshot.CreatedAt, snapshot.UpdatedAt,
		)
		if err != nil {
			return err
		}
		return insertTags(ctx, tx, snapshot.ID, snapshot.TagList)
	})
	return translateConflict(err)
}

func (r *MySQLRepository) Update(ctx context.Context, article domain.Article) (bool, error) {
	snapshot := article.Snapshot()
	updated := false
	err := mysql.WithTransaction(ctx, r.db, func(tx mysql.Querier) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE articles
			    SET slug = ?, title = ?, description = ?, body = ?, updated_at = ?
			  WHERE id = ?`,
			snapshot.Slug, snapshot.Title, snapshot.Description, snapshot.Body, snapshot.UpdatedAt, snapshot.ID,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return nil
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM article_tags WHERE article_id = ?", snapshot.ID); err != nil {
			return err
		}
		updated = true
		return insertTags(ctx, tx, snapshot.ID, snapshot.TagList)
	})
	if err != nil {
		return false, translateConflict(err)
	}
	return updated, nil
}

func (r *MySQLRepository) FindBySlug(ctx context.Context, slug string) (domain.Article, bool, error) {
	var snapshot domain.Snapshot
	err := r.db.QueryRowContext(ctx,
		`SELECT id, author_id, slug, title, description, body, created_at, updated_at
		   FROM articles
		  WHERE slug = ?`,
		slug,
	).Scan(
		&snapshot.ID, &snapshot.AuthorID, &snapshot.Slug, &snapshot.Title, &snapshot.Description,
		&snapshot.Body, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Article{}, false, nil
	}
	if err != nil {
		return domain.Article{}, false, err
	}

	tags, err := loadTags(ctx, r.db, []string{snapshot.ID})
	if err != nil {
		return domain.Article{}, false, err
	}
	snapshot.TagList = tagsFor(tags, snapshot.ID)
	return domain.Restore(snapshot), true, nil
}

func (r *MySQLRepository) FindViewBySlug(ctx context.Context, slug, viewerID string) (domain.View, bool, error) {
	var view domain.View
	err := r.db.QueryRowContext(ctx,
		"SELECT "+viewColumns+", a.body FROM articles a WHERE a.slug = ?",
		nullable(viewerID), slug,
	).Scan(
		&view.ID, &view.AuthorID, &view.Slug, &view.Title, &view.Description,
		&view.CreatedAt, &view.UpdatedAt, &view.FavoritesCount, &view.Favorited, &view.Body,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.View{}, false, nil
	}
	if err != nil {
		return domain.View{}, false, err
	}

	tags, err := loadTags(ctx, r.db, []string{view.ID})
	if err != nil {
		return domain.View{}, false, err
	}
	view.TagList = tagsFor(tags, view.ID)
	return normalizeView(view), true, nil
}

func (r *MySQLRepository) FindPage(ctx context.Context, query domain.PageQuery) (domain.Page, error) {
	conditions := []string{}
	args := []any{}

	if query.Tag != nil {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM article_tags t WHERE t.article_id = a.id AND t.tag = ?)")
		args = append(args, *query.Tag)
	}
	if query.AuthorID != "" {
		conditions = append(conditions, "a.author_id = ?")
		args = append(args, query.AuthorID)
	}
	if query.AuthorIDs != nil {
		if len(query.AuthorIDs) == 0 {
			conditions = append(conditions, "1 = 0")
		} else {
			conditions = append(conditions, "a.author_id IN ("+mysql.Placeholders(len(query.AuthorIDs))+")")
			args = append(args, mysql.StringArgs(query.AuthorIDs)...)
		}
	}
	if query.FavoritedByID != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM favorites f WHERE f.article_id = a.id AND f.user_id = ?)")
		args = append(args, query.FavoritedByID)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles a "+where, args...).Scan(&total); err != nil {
		return domain.Page{}, err
	}

	listArgs := make([]any, 0, len(args)+3)
	listArgs = append(listArgs, nullable(query.ViewerID))
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, query.Limit, query.Offset)

	rows, err := r.db.QueryContext(ctx,
		"SELECT "+viewColumns+" FROM articles a "+where+
			" ORDER BY a.created_at DESC, a.seq DESC LIMIT ? OFFSET ?",
		listArgs...,
	)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()

	items := []domain.View{}
	ids := []string{}
	for rows.Next() {
		var view domain.View
		if err := rows.Scan(
			&view.ID, &view.AuthorID, &view.Slug, &view.Title, &view.Description,
			&view.CreatedAt, &view.UpdatedAt, &view.FavoritesCount, &view.Favorited,
		); err != nil {
			return domain.Page{}, err
		}
		items = append(items, view)
		ids = append(ids, view.ID)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, err
	}
	rows.Close()

	tags, err := loadTags(ctx, r.db, ids)
	if err != nil {
		return domain.Page{}, err
	}
	for index := range items {
		items[index].TagList = tagsFor(tags, items[index].ID)
		items[index] = normalizeView(items[index])
	}
	return domain.Page{Items: items, Total: total}, nil
}

func (r *MySQLRepository) SlugTaken(ctx context.Context, slug, excludeID string) (bool, error) {
	statement := "SELECT EXISTS (SELECT 1 FROM articles WHERE slug = ?"
	args := []any{slug}
	if excludeID != "" {
		statement += " AND id <> ?"
		args = append(args, excludeID)
	}
	statement += ")"

	var taken bool
	err := r.db.QueryRowContext(ctx, statement, args...).Scan(&taken)
	return taken, err
}

func (r *MySQLRepository) DeleteByID(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM articles WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *MySQLRepository) Favorite(ctx context.Context, articleID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO favorites (user_id, article_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE user_id = user_id",
		userID, articleID,
	)
	return err
}

func (r *MySQLRepository) Unfavorite(ctx context.Context, articleID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM favorites WHERE user_id = ? AND article_id = ?",
		userID, articleID,
	)
	return err
}

func (r *MySQLRepository) PopularTags(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT tag FROM article_tags
		  GROUP BY tag
		  ORDER BY COUNT(*) DESC, tag ASC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func loadTags(ctx context.Context, db mysql.Querier, articleIDs []string) (map[string][]string, error) {
	tags := map[string][]string{}
	if len(articleIDs) == 0 {
		return tags, nil
	}
	rows, err := db.QueryContext(ctx,
		"SELECT article_id, tag FROM article_tags WHERE article_id IN ("+mysql.Placeholders(len(articleIDs))+
			") ORDER BY article_id, position",
		mysql.StringArgs(articleIDs)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var articleID, tag string
		if err := rows.Scan(&articleID, &tag); err != nil {
			return nil, err
		}
		tags[articleID] = append(tags[articleID], tag)
	}
	return tags, rows.Err()
}

func tagsFor(tags map[string][]string, articleID string) []string {
	if list, ok := tags[articleID]; ok {
		return list
	}
	return []string{}
}

func insertTags(ctx context.Context, tx mysql.Querier, articleID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	rows := make([]string, len(tags))
	args := make([]any, 0, len(tags)*3)
	for position, tag := range tags {
		rows[position] = "(?, ?, ?)"
		args = append(args, articleID, position, tag)
	}
	_, err := tx.ExecContext(ctx,
		"INSERT INTO article_tags (article_id, position, tag) VALUES "+strings.Join(rows, ", "),
		args...,
	)
	return err
}

func normalizeView(view domain.View) domain.View {
	view.CreatedAt = view.CreatedAt.UTC()
	view.UpdatedAt = view.UpdatedAt.UTC()
	if view.TagList == nil {
		view.TagList = []string{}
	}
	return view
}

func translateConflict(err error) error {
	if err != nil && mysql.ConstraintName(err) == slugConstraint {
		return domain.ErrSlugTaken
	}
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
