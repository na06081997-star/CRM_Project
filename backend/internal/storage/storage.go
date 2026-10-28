// Package storage wraps the S3-compatible object store (Garage).
// Objects are never exposed directly: the backend streams them to clients,
// so Garage stays on the private Docker network.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrDisabled = errors.New("object storage is not configured")

type Store interface {
	Put(ctx context.Context, prefix, filename, contentType string, r io.Reader, size int64) (key string, err error)
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
	Ready(ctx context.Context) error
}

type ObjectInfo struct {
	ContentType string
	Size        int64
	ModTime     time.Time
}

type S3 struct {
	client *minio.Client
	bucket string
}

func NewS3(endpoint, accessKey, secretKey, bucket, region string) (Store, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" {
		return disabled{}, nil
	}
	secure := strings.HasPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	c, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       secure,
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	return &S3{client: c, bucket: bucket}, nil
}

func (s *S3) Put(ctx context.Context, prefix, filename, contentType string, r io.Reader, size int64) (string, error) {
	ext := strings.ToLower(path.Ext(filename))
	key := fmt.Sprintf("%s/%s/%s%s", prefix, time.Now().UTC().Format("2006/01"), uuid.NewString(), ext)
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("upload %s: %w", key, err)
	}
	return key, nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	st, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, ObjectInfo{}, err
	}
	return obj, ObjectInfo{ContentType: st.ContentType, Size: st.Size, ModTime: st.LastModified}, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3) Ready(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q missing", s.bucket)
	}
	return nil
}

type disabled struct{}

func (disabled) Put(context.Context, string, string, string, io.Reader, int64) (string, error) {
	return "", ErrDisabled
}
func (disabled) Get(context.Context, string) (io.ReadCloser, ObjectInfo, error) {
	return nil, ObjectInfo{}, ErrDisabled
}
func (disabled) Delete(context.Context, string) error { return ErrDisabled }
func (disabled) Ready(context.Context) error          { return ErrDisabled }
