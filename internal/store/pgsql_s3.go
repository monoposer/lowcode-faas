package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/jackc/pgx/v5/stdlib"

	"lowcode-faas/internal/config"
	"lowcode-faas/internal/envutil"
	"lowcode-faas/internal/model"
)

var (
	appDB       *sql.DB
	s3Client    *s3.Client
	dbS3WriteMu sync.Mutex
)

const migratePGV2 = `
CREATE TABLE IF NOT EXISTS collection_paths (
	path TEXT PRIMARY KEY NOT NULL,
	env_json TEXT,
	created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS functions (
	collection_path TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	language TEXT NOT NULL,
	env_json TEXT,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (collection_path, name)
);
CREATE TABLE IF NOT EXISTS function_versions (
	id BIGSERIAL PRIMARY KEY,
	collection_path TEXT NOT NULL DEFAULT '',
	function_name TEXT NOT NULL,
	version INTEGER NOT NULL,
	s3_key TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	UNIQUE (collection_path, function_name, version),
	FOREIGN KEY (collection_path, function_name) REFERENCES functions(collection_path, name) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_function_versions_lookup ON function_versions(collection_path, function_name);
`

func initPGS3(ctx context.Context) error {
	if strings.TrimSpace(config.PostgresDSN) == "" {
		return fmt.Errorf("db_s3 mode requires PostgresDSN")
	}
	if strings.TrimSpace(config.S3Bucket) == "" {
		return fmt.Errorf("db_s3 mode requires S3Bucket")
	}
	db, err := sql.Open("pgx", config.PostgresDSN)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("postgres ping: %w", err)
	}
	if _, err := db.ExecContext(ctx, migratePGV2); err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate v2: %w", err)
	}
	if err := migrateLegacyPGS3(ctx, db); err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate legacy: %w", err)
	}
	appDB = db
	cli, err := newS3Client(ctx)
	if err != nil {
		_ = db.Close()
		appDB = nil
		return err
	}
	s3Client = cli
	return nil
}

func migrateLegacyPGS3(ctx context.Context, db *sql.DB) error {
	var has bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'functions' AND column_name = 'collection_path'
		)`).Scan(&has)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	// Old schema: functions(name PK), function_versions(function_name -> name)
	stmts := []string{
		`ALTER TABLE IF EXISTS function_versions DROP CONSTRAINT IF EXISTS function_versions_function_name_fkey`,
		`ALTER TABLE IF EXISTS function_versions DROP CONSTRAINT IF EXISTS function_versions_function_name_version_key`,
		`ALTER TABLE functions ADD COLUMN IF NOT EXISTS collection_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE function_versions ADD COLUMN IF NOT EXISTS collection_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE functions DROP CONSTRAINT IF EXISTS functions_pkey`,
		`ALTER TABLE functions ADD PRIMARY KEY (collection_path, name)`,
		`ALTER TABLE function_versions ADD CONSTRAINT function_versions_function_name_version_key UNIQUE (collection_path, function_name, version)`,
		`ALTER TABLE function_versions ADD CONSTRAINT function_versions_function_name_fkey
			FOREIGN KEY (collection_path, function_name) REFERENCES functions(collection_path, name) ON DELETE CASCADE`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

func newS3Client(ctx context.Context) (*s3.Client, error) {
	region := strings.TrimSpace(config.S3Region)
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if strings.TrimSpace(config.S3AccessKey) != "" || strings.TrimSpace(config.S3SecretKey) != "" {
		if strings.TrimSpace(config.S3AccessKey) == "" || strings.TrimSpace(config.S3SecretKey) == "" {
			return nil, fmt.Errorf("S3AccessKey and S3SecretKey must both be set for static credentials")
		}
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(config.S3AccessKey, config.S3SecretKey, "")))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if ep := strings.TrimSpace(config.S3Endpoint); ep != "" {
			o.BaseEndpoint = aws.String(ep)
		}
		o.UsePathStyle = config.S3UsePathStyle
	}), nil
}

func s3ObjectKey(collectionPath, name string, version int, ext string) string {
	cp := strings.Trim(collectionPath, "/")
	if cp == "" {
		return fmt.Sprintf("functions/%s/v%d%s", name, version, ext)
	}
	return fmt.Sprintf("functions/%s/%s/v%d%s", filepath.ToSlash(cp), name, version, ext)
}

func pathPrefixes(p string) []string {
	segs := strings.Split(p, "/")
	out := make([]string, 0, len(segs))
	for i := range segs {
		out = append(out, strings.Join(segs[:i+1], "/"))
	}
	return out
}

func createCollectionPGS3(ctx context.Context, path string) (*model.CollectionListItem, error) {
	dbS3WriteMu.Lock()
	defer dbS3WriteMu.Unlock()
	var exists int
	if err := appDB.QueryRowContext(ctx, `SELECT 1 FROM collection_paths WHERE path = $1`, path).Scan(&exists); err == nil {
		return nil, ErrCollectionExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	now := time.Now().UTC()
	prefixes := pathPrefixes(path)
	for _, p := range prefixes {
		_, err := appDB.ExecContext(ctx, `
			INSERT INTO collection_paths (path, env_json, created_at) VALUES ($1, NULL, $2)
			ON CONFLICT(path) DO NOTHING
		`, p, now)
		if err != nil {
			return nil, fmt.Errorf("insert collection path %q: %w", p, err)
		}
	}
	var created time.Time
	err := appDB.QueryRowContext(ctx, `SELECT created_at FROM collection_paths WHERE path = $1`, path).Scan(&created)
	if err != nil {
		return nil, err
	}
	return &model.CollectionListItem{Path: path, CreatedAt: created.UTC()}, nil
}

func collectionPathExistsPGS3(ctx context.Context, path string) (bool, error) {
	var n int
	err := appDB.QueryRowContext(ctx, `SELECT 1 FROM collection_paths WHERE path = $1 LIMIT 1`, path).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getCollectionEnvPGS3(ctx context.Context, path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	var raw sql.NullString
	err := appDB.QueryRowContext(ctx, `SELECT env_json FROM collection_paths WHERE path = $1`, path).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !raw.Valid || raw.String == "" || raw.String == "null" {
		return nil, nil
	}
	var m map[string]string
	if json.Unmarshal([]byte(raw.String), &m) != nil {
		return nil, nil
	}
	return m, nil
}

func setCollectionEnvPGS3(ctx context.Context, path string, env map[string]string) error {
	if path == "" {
		return fmt.Errorf("cannot set env on root collection in db_s3")
	}
	envJSON, err := json.Marshal(env)
	if err != nil {
		return err
	}
	res, err := appDB.ExecContext(ctx, `
		UPDATE collection_paths SET env_json = $2 WHERE path = $1
	`, path, string(envJSON))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func listCollectionsPGS3(ctx context.Context, prefix string) ([]*model.CollectionListItem, error) {
	var rows *sql.Rows
	var err error
	if prefix == "" {
		rows, err = appDB.QueryContext(ctx, `SELECT path, created_at FROM collection_paths ORDER BY path`)
	} else {
		rows, err = appDB.QueryContext(ctx, `
			SELECT path, created_at FROM collection_paths
			WHERE path = $1 OR path LIKE $2
			ORDER BY path
		`, prefix, prefix+"/%")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.CollectionListItem
	for rows.Next() {
		var p string
		var t time.Time
		if err := rows.Scan(&p, &t); err != nil {
			return nil, err
		}
		out = append(out, &model.CollectionListItem{Path: p, CreatedAt: t.UTC()})
	}
	return out, rows.Err()
}

func createFunctionPGS3(ctx context.Context, collectionPath, name, language, source string, env map[string]string) (*model.Function, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("function name is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid function name")
	}
	if collectionPath != "" {
		ok, err := collectionPathExistsPGS3(ctx, collectionPath)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrCollectionNotFound
		}
	}
	ext, err := extForLanguage(language)
	if err != nil {
		return nil, err
	}
	dbS3WriteMu.Lock()
	defer dbS3WriteMu.Unlock()
	now := time.Now().UTC()
	envJSON, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	_, err = appDB.ExecContext(ctx, `
		INSERT INTO functions (collection_path, name, language, env_json, updated_at) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT(collection_path, name) DO UPDATE SET
			language = EXCLUDED.language,
			env_json = EXCLUDED.env_json,
			updated_at = EXCLUDED.updated_at
	`, collectionPath, name, language, string(envJSON), now)
	if err != nil {
		return nil, fmt.Errorf("upsert function meta: %w", err)
	}
	var maxVer int
	if err := appDB.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM function_versions WHERE collection_path = $1 AND function_name = $2`,
		collectionPath, name).Scan(&maxVer); err != nil {
		return nil, fmt.Errorf("query version: %w", err)
	}
	nextVer := maxVer + 1
	key := s3ObjectKey(collectionPath, name, nextVer, ext)
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(source)),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 put object: %w", err)
	}
	_, err = appDB.ExecContext(ctx, `
		INSERT INTO function_versions (collection_path, function_name, version, s3_key, created_at) VALUES ($1,$2,$3,$4,$5)
	`, collectionPath, name, nextVer, key, now)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}
	depID := model.FormatDeploymentID(collectionPath, name, nextVer)
	return &model.Function{
		ID:             functionQualifiedID(collectionPath, name),
		CollectionPath: collectionPath,
		Name:           name,
		Language:       language,
		SourceCode:     source,
		Env:            envutil.CloneStringMap(env),
		CreatedAt:      now,
		Version:        nextVer,
		DeploymentID:   depID,
	}, nil
}

func functionQualifiedID(collectionPath, name string) string {
	if collectionPath != "" {
		return collectionPath + "/" + name
	}
	return name
}

func getFunctionByNamePGS3(ctx context.Context, collectionPath, name string, version *int) (*model.Function, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNotFound
	}
	var lang, envJSON, s3Key string
	var ver int
	var createdAt time.Time
	var q string
	var args []any
	if version != nil {
		q = `
		SELECT f.language, f.env_json, v.s3_key, v.created_at, v.version
		FROM functions f
		JOIN function_versions v ON v.collection_path = f.collection_path AND v.function_name = f.name
		WHERE f.collection_path = $1 AND f.name = $2 AND v.version = $3`
		args = []any{collectionPath, name, *version}
	} else {
		q = `
		SELECT f.language, f.env_json, v.s3_key, v.created_at, v.version
		FROM functions f
		JOIN function_versions v ON v.collection_path = f.collection_path AND v.function_name = f.name
		WHERE f.collection_path = $1 AND f.name = $2
		ORDER BY v.version DESC
		LIMIT 1`
		args = []any{collectionPath, name}
	}
	err = appDB.QueryRowContext(ctx, q, args...).Scan(&lang, &envJSON, &s3Key, &createdAt, &ver)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %w", err)
	}
	defer out.Body.Close()
	max := config.SourceFetchMaxBytes
	if max <= 0 {
		max = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(out.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("s3 object exceeds max size (%d bytes)", max)
	}
	var env map[string]string
	if envJSON != "" && envJSON != "null" {
		_ = json.Unmarshal([]byte(envJSON), &env)
	}
	return &model.Function{
		ID:             functionQualifiedID(collectionPath, name),
		CollectionPath: collectionPath,
		Name:           name,
		Language:       lang,
		SourceCode:     string(body),
		Env:            env,
		CreatedAt:      createdAt.UTC(),
		Version:        ver,
		DeploymentID:   model.FormatDeploymentID(collectionPath, name, ver),
	}, nil
}

func listFunctionsPGS3(ctx context.Context, collectionPath string) ([]*model.FunctionListItem, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	rows, err := appDB.QueryContext(ctx, `
		SELECT f.collection_path, f.name, f.language, f.updated_at,
			(SELECT MAX(version) FROM function_versions v
			 WHERE v.collection_path = f.collection_path AND v.function_name = f.name) AS ver
		FROM functions f
		WHERE f.collection_path = $1
		ORDER BY f.name
	`, collectionPath)
	if err != nil {
		return nil, fmt.Errorf("list functions: %w", err)
	}
	defer rows.Close()
	var out []*model.FunctionListItem
	for rows.Next() {
		var cp, fnm, lang string
		var updatedAt time.Time
		var ver sql.NullInt64
		if err := rows.Scan(&cp, &fnm, &lang, &updatedAt, &ver); err != nil {
			return nil, err
		}
		v := 0
		if ver.Valid {
			v = int(ver.Int64)
		}
		out = append(out, &model.FunctionListItem{
			ID:             functionQualifiedID(cp, fnm),
			CollectionPath: cp,
			Name:           fnm,
			Language:       lang,
			Version:        v,
			CreatedAt:      updatedAt.UTC(),
		})
	}
	return out, rows.Err()
}

func listFunctionVersionsPGS3(ctx context.Context, collectionPath, name string) ([]model.FunctionVersionItem, error) {
	collectionPath, err := NormalizeCollectionPath(collectionPath)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("function name is required")
	}
	rows, err := appDB.QueryContext(ctx, `
		SELECT version, created_at FROM function_versions
		WHERE collection_path = $1 AND function_name = $2
		ORDER BY version DESC
	`, collectionPath, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FunctionVersionItem
	for rows.Next() {
		var v int
		var t time.Time
		if err := rows.Scan(&v, &t); err != nil {
			return nil, err
		}
		out = append(out, model.FunctionVersionItem{Version: v, CreatedAt: t.UTC()})
	}
	return out, rows.Err()
}