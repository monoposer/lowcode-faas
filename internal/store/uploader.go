package store

import (
	"context"

	"lowcode-faas/internal/config"
)

// NewUploaderFromConfig always returns an S3-compatible OSS uploader.
func NewUploaderFromConfig(ctx context.Context, cfg config.Config) (Uploader, error) {
	return NewS3Uploader(ctx, S3Config{
		Endpoint:     cfg.S3Endpoint,
		Region:       cfg.S3Region,
		Bucket:       cfg.S3Bucket,
		AccessKey:    cfg.S3AccessKey,
		SecretKey:    cfg.S3SecretKey,
		UsePathStyle: cfg.S3UsePathStyle,
		PublicBase:   cfg.S3PublicBase,
	})
}
