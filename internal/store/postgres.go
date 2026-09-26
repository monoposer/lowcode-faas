package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monoposer/lowcode-faas/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	s := &Postgres{pool: pool}
	if err := s.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Postgres) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Postgres) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrateSQL(s.namespace(), `
CREATE TABLE IF NOT EXISTS actions (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  label VARCHAR(256) NOT NULL DEFAULT '',
  "group" VARCHAR(128) NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  source_type VARCHAR(64) NOT NULL DEFAULT 'TYPESCRIPT',
  source_url TEXT NOT NULL DEFAULT '',
  js_url TEXT NOT NULL DEFAULT '',
  etag VARCHAR(64) NOT NULL DEFAULT '',
  async BOOLEAN NOT NULL DEFAULT FALSE,
  timeout INTEGER NOT NULL DEFAULT 60,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS actions_group_name_uidx
  ON actions ("group", name) WHERE deleted_at IS NULL;
`))
	return err
}

const actionCols = `id, name, label, "group", description, source_type,
  source_url, js_url, etag, async, timeout, version, created_at, updated_at, deleted_at`

func scanAction(row pgx.Row) (*model.Action, error) {
	var a model.Action
	err := row.Scan(
		&a.ID, &a.Name, &a.Label, &a.Group, &a.Description, &a.SourceType,
		&a.SourceURL, &a.JsURL, &a.Etag, &a.Async, &a.Timeout, &a.Version, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// ListOpts filters the action list. Empty fields are ignored.
type ListOpts struct {
	Group string
	Q     string // case-insensitive match on name, label, group, description
}

// List returns actions matching opts, ordered by group then name.
func (s *Postgres) List(ctx context.Context, opts ListOpts) ([]*model.Action, error) {
	where := []string{`deleted_at IS NULL`}
	args := make([]any, 0, 2)
	if opts.Group != "" {
		args = append(args, opts.Group)
		where = append(where, fmt.Sprintf(`"group" = $%d`, len(args)))
	}
	if q := strings.TrimSpace(opts.Q); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(
			`(lower(name) LIKE $%d OR lower(label) LIKE $%d OR lower("group") LIKE $%d OR lower(description) LIKE $%d)`,
			n, n, n, n,
		))
	}
	sql := `SELECT ` + actionCols + ` FROM actions WHERE ` + strings.Join(where, ` AND `) +
		` ORDER BY "group" ASC, name ASC`
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.Action, 0)
	for rows.Next() {
		a, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Postgres) GetByGroupName(ctx context.Context, group, name string) (*model.Action, error) {
	return scanAction(s.pool.QueryRow(ctx, `
SELECT `+actionCols+` FROM actions
WHERE "group" = $1 AND name = $2 AND deleted_at IS NULL`, group, name))
}

func (s *Postgres) GetByID(ctx context.Context, id int64) (*model.Action, error) {
	return scanAction(s.pool.QueryRow(ctx, `
SELECT `+actionCols+` FROM actions
WHERE id = $1 AND deleted_at IS NULL`, id))
}

func (s *Postgres) Create(ctx context.Context, in model.CreateAction) (*model.Action, error) {
	if in.Timeout <= 0 {
		in.Timeout = 60
	}
	if in.SourceType == "" {
		in.SourceType = "TYPESCRIPT"
	}
	if in.Label == "" {
		in.Label = in.Name
	}
	return scanAction(s.pool.QueryRow(ctx, `
INSERT INTO actions (
  name, label, "group", description, source_type,
  source_url, js_url, etag, async, timeout, version
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1)
RETURNING `+actionCols, in.Name, in.Label, in.Group, in.Description, in.SourceType,
		in.SourceURL, in.JsURL, in.Etag, in.Async, in.Timeout))
}

func (s *Postgres) Update(ctx context.Context, id int64, in model.UpdateAction) (*model.Action, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name, label, group, desc := existing.Name, existing.Label, existing.Group, existing.Description
	st, srcURL, jsURL, etag := existing.SourceType, existing.SourceURL, existing.JsURL, existing.Etag
	async, timeout, version := existing.Async, existing.Timeout, existing.Version
	if in.Name != nil && *in.Name != "" {
		name = *in.Name
	}
	if in.Label != nil {
		label = *in.Label
	}
	if in.Group != nil {
		group = *in.Group
	}
	if in.Description != nil {
		desc = *in.Description
	}
	if in.SourceType != nil {
		st = *in.SourceType
	}
	if in.SourceURL != nil {
		srcURL = *in.SourceURL
	}
	if in.JsURL != nil {
		jsURL = *in.JsURL
	}
	if in.Etag != nil {
		etag = *in.Etag
	}
	if in.Async != nil {
		async = *in.Async
	}
	if in.Timeout != nil {
		timeout = *in.Timeout
	}
	version++
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
UPDATE actions SET
  name=$1, label=$2, "group"=$3, description=$4, source_type=$5,
  source_url=$6, js_url=$7, etag=$8, async=$9, timeout=$10,
  version=$11, updated_at=$12
WHERE id=$13 AND deleted_at IS NULL`,
		name, label, group, desc, st, srcURL, jsURL, etag, async, timeout, version, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *Postgres) SoftDelete(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE actions SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Postgres) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres not initialized")
	}
	return s.pool.Ping(ctx)
}

// IsUniqueViolation reports whether err is a Postgres unique_violation (23505).
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
