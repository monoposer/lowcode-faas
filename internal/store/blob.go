package store

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Uploader stores TypeScript source and compiled JS in S3-compatible OSS (rustfs / MinIO / AWS).
type Uploader interface {
	Put(ctx context.Context, key string, body io.Reader) (publicURL string, err error)
	GetBytes(ctx context.Context, publicURL string) ([]byte, error)
}

// S3Uploader stores blobs under action-js/ prefix via the S3 API.
type S3Uploader struct {
	client *s3.Client
	bucket string
	// PublicBase is the HTTP base for returned URLs (path-style: endpoint/bucket).
	PublicBase string
}

type S3Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
	PublicBase   string
}

func NewS3Uploader(ctx context.Context, cfg S3Config) (*S3Uploader, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = cfg.UsePathStyle
	})
	public := strings.TrimRight(cfg.PublicBase, "/")
	if public == "" {
		public = strings.TrimRight(cfg.Endpoint, "/") + "/" + cfg.Bucket
	}
	u := &S3Uploader{client: client, bucket: cfg.Bucket, PublicBase: public}
	if err := u.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *S3Uploader) ensureBucket(ctx context.Context) error {
	_, err := u.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(u.bucket)})
	if err == nil {
		return nil
	}
	_, err = u.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(u.bucket)})
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "already") || strings.Contains(msg, "bucketalreadyownedbyyou") {
			return nil
		}
		return fmt.Errorf("ensure s3 bucket %q: %w", u.bucket, err)
	}
	return nil
}

func (u *S3Uploader) Put(ctx context.Context, key string, body io.Reader) (string, error) {
	key = normalizeS3Key(key)
	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", u.PublicBase, key), nil
}

func (u *S3Uploader) GetBytes(ctx context.Context, publicURL string) ([]byte, error) {
	key := keyFromS3URL(publicURL, u.bucket)
	if key == "" {
		return fetchHTTP(ctx, publicURL)
	}
	out, err := u.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func normalizeS3Key(key string) string {
	key = strings.TrimPrefix(key, "/")
	if !strings.HasPrefix(key, "action-js/") {
		key = "action-js/" + key
	}
	return key
}

func keyFromS3URL(publicURL, bucket string) string {
	if i := strings.Index(publicURL, "action-js/"); i >= 0 {
		return publicURL[i:]
	}
	prefix := "s3://" + bucket + "/"
	if strings.HasPrefix(publicURL, prefix) {
		return strings.TrimPrefix(publicURL, prefix)
	}
	return ""
}

func fetchHTTP(ctx context.Context, publicURL string) ([]byte, error) {
	if publicURL == "" || !strings.Contains(publicURL, "://") {
		return nil, fmt.Errorf("cannot resolve artifact url: %s", publicURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s: status %d", publicURL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

var _ Uploader = (*S3Uploader)(nil)
