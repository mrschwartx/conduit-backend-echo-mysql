package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/mysql"
)

const userColumns = "id, username, email, password_hash, bio, image, created_at, updated_at"

var fieldByConstraint = map[string]string{
	"users_email_unique":    "email",
	"users_username_unique": "username",
}

type MySQLRepository struct {
	db mysql.Querier
}

func NewMySQLRepository(db mysql.Querier) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *MySQLRepository) Add(ctx context.Context, user domain.User) error {
	snapshot := user.Snapshot()
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO users ("+userColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		snapshot.ID, snapshot.Username, snapshot.Email, snapshot.PasswordHash,
		snapshot.Bio, snapshot.Image, snapshot.CreatedAt, snapshot.UpdatedAt,
	)
	return translateConflict(err)
}

func (r *MySQLRepository) Update(ctx context.Context, user domain.User) (bool, error) {
	snapshot := user.Snapshot()
	result, err := r.db.ExecContext(ctx,
		`UPDATE users
		    SET username = ?,
		        email = ?,
		        password_hash = ?,
		        bio = ?,
		        image = ?,
		        updated_at = ?
		  WHERE id = ?`,
		snapshot.Username, snapshot.Email, snapshot.PasswordHash,
		snapshot.Bio, snapshot.Image, snapshot.UpdatedAt, snapshot.ID,
	)
	if err != nil {
		return false, translateConflict(err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (r *MySQLRepository) FindByID(ctx context.Context, id string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE id = ?", id)
}

func (r *MySQLRepository) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE LOWER(email) = LOWER(?)", email)
}

func (r *MySQLRepository) FindByUsername(ctx context.Context, username string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE LOWER(username) = LOWER(?)", username)
}

func (r *MySQLRepository) FindByIDs(ctx context.Context, ids []string) ([]domain.User, error) {
	users := []domain.User{}
	if len(ids) == 0 {
		return users, nil
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE id IN ("+mysql.Placeholders(len(ids))+")",
		mysql.StringArgs(ids)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *MySQLRepository) ExistsByID(ctx context.Context, id string) (bool, error) {
	var marker int
	err := r.db.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", id).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *MySQLRepository) FindTakenFields(ctx context.Context, query domain.TakenQuery) ([]string, error) {
	taken := []string{}

	emailTaken, err := r.fieldTaken(ctx, "email", query.Email, query.ExcludeID)
	if err != nil {
		return nil, err
	}
	if emailTaken {
		taken = append(taken, "email")
	}

	usernameTaken, err := r.fieldTaken(ctx, "username", query.Username, query.ExcludeID)
	if err != nil {
		return nil, err
	}
	if usernameTaken {
		taken = append(taken, "username")
	}
	return taken, nil
}

func (r *MySQLRepository) fieldTaken(ctx context.Context, column, value, excludeID string) (bool, error) {
	if value == "" {
		return false, nil
	}
	statement := "SELECT EXISTS (SELECT 1 FROM users WHERE LOWER(" + column + ") = LOWER(?)"
	args := []any{value}
	if excludeID != "" {
		statement += " AND id <> ?"
		args = append(args, excludeID)
	}
	statement += ")"

	var taken bool
	err := r.db.QueryRowContext(ctx, statement, args...).Scan(&taken)
	return taken, err
}

func (r *MySQLRepository) findOne(ctx context.Context, statement string, args ...any) (domain.User, bool, error) {
	user, err := scanUser(r.db.QueryRowContext(ctx, statement, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, err
	}
	return user, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (domain.User, error) {
	var snapshot domain.Snapshot
	err := row.Scan(
		&snapshot.ID, &snapshot.Username, &snapshot.Email, &snapshot.PasswordHash,
		&snapshot.Bio, &snapshot.Image, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, err
	}
	return domain.Restore(snapshot), nil
}

func translateConflict(err error) error {
	if err == nil {
		return nil
	}
	if field, ok := fieldByConstraint[mysql.ConstraintName(err)]; ok {
		return sharedomain.NewConflictError(field)
	}
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
