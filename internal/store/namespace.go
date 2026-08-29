package store

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// namespaceFromPool reads search_path from the DSN (auth-cn DB_NAMESPACE analog).
// Empty means public. SQL must not hardcode a schema name.
func namespaceFromPool(pool *pgxpool.Pool) string {
	if pool == nil || pool.Config() == nil {
		return ""
	}
	return normalizeNamespace(pool.Config().ConnConfig.RuntimeParams["search_path"])
}

func namespaceFromDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	return normalizeNamespace(u.Query().Get("search_path"))
}

func normalizeNamespace(searchPath string) string {
	sp := strings.TrimSpace(searchPath)
	if sp == "" {
		return ""
	}
	first := strings.Split(sp, ",")[0]
	first = strings.Trim(strings.TrimSpace(first), `"'`)
	if first == "" || !identRe.MatchString(first) {
		return ""
	}
	return first
}

func ensureNamespaceSQL(name string) string {
	name = normalizeNamespace(name)
	if name == "" || name == "public" {
		return ""
	}
	ident := pgx.Identifier{name}.Sanitize()
	return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s;\nSET search_path TO %s;\n", ident, ident)
}

func migrateSQL(namespace, body string) string {
	return ensureNamespaceSQL(namespace) + body
}

func (s *Postgres) namespace() string {
	return namespaceFromPool(s.pool)
}
