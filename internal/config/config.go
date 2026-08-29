package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Listen       string // meta listen addr
	WorkerListen string // worker listen addr
	MetaURL      string // worker → meta base URL (e.g. http://127.0.0.1:8080)

	PostgresDSN string

	// S3-compatible OSS (required on meta). source_url / js_url always point here.
	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
	S3UsePathStyle bool
	S3PublicBase   string // optional; default endpoint/bucket

	// Worker JS LRU (entries keyed by group+name; invalidated via etag / If-None-Match).
	JSCacheSize int

	JSMock bool
}

func Load() Config {
	endpoint := env("LOWCODE_FAAS_S3_ENDPOINT", "http://127.0.0.1:9000")
	bucket := env("LOWCODE_FAAS_S3_BUCKET", "lowcode-faas")
	return Config{
		Listen:         env("LOWCODE_FAAS_LISTEN", ":8080"),
		WorkerListen:   env("LOWCODE_FAAS_WORKER_LISTEN", ":9090"),
		MetaURL:        strings.TrimRight(env("LOWCODE_FAAS_META_URL", "http://127.0.0.1:8080"), "/"),
		PostgresDSN:    env("LOWCODE_FAAS_POSTGRES_DSN", "postgres://faas:faas@localhost:5433/lowcode?sslmode=disable&search_path=faas"),
		S3Endpoint:     endpoint,
		S3Region:       env("LOWCODE_FAAS_S3_REGION", "us-east-1"),
		S3Bucket:       bucket,
		S3AccessKey:    env("LOWCODE_FAAS_S3_ACCESS_KEY", "rustfsadmin"),
		S3SecretKey:    env("LOWCODE_FAAS_S3_SECRET_KEY", "rustfsadmin"),
		S3UsePathStyle: envBool("LOWCODE_FAAS_S3_USE_PATH_STYLE", true),
		S3PublicBase:   strings.TrimRight(env("LOWCODE_FAAS_S3_PUBLIC_BASE", endpoint+"/"+bucket), "/"),
		JSCacheSize:    envInt("LOWCODE_FAAS_JS_CACHE_SIZE", 128),
		JSMock:         envBool("FAAS_JS_MOCK", false),
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
