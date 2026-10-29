/*
Copyright The Alpheus Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config says how to reach a bucket. It comes from spec.storage and the
// Secret it names.
type S3Config struct {
	// Endpoint is the URL of the S3 API, http:// or https://.
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	// CAFile is a PEM file of CA certificates to trust for https, in
	// addition to the system's. Empty means the system's only.
	CAFile string
}

// S3 is a Store in an S3-compatible bucket.
type S3 struct {
	client *minio.Client
	bucket string
}

// NewS3 connects to the bucket. It makes no request; the first call does.
func NewS3(cfg S3Config) (*S3, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("s3 endpoint %q: want http(s)://host[:port]", cfg.Endpoint)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("s3 endpoint %q: must not have a path", cfg.Endpoint)
	}
	transport, err := transport(cfg.CAFile)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    u.Scheme == "https",
		Region:    cfg.Region,
		Transport: transport,
		// Path-style requests (endpoint/bucket/key) work with every
		// S3-compatible store and need no DNS per bucket.
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	return &S3{client: client, bucket: cfg.Bucket}, nil
}

func transport(caFile string) (http.RoundTripper, error) {
	t, err := minio.DefaultTransport(true)
	if err != nil {
		return nil, fmt.Errorf("s3 transport: %w", err)
	}
	if caFile == "" {
		return t, nil
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("s3 CA file: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("s3 CA file %s: no certificates", caFile)
	}
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	t.TLSClientConfig.RootCAs = pool
	return t, nil
}

func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	if _, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, mapErr(err))
	}
	// GetObject is lazy; Stat makes the request, so a missing object is
	// reported here and not on the first Read.
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, fmt.Errorf("get %s: %w", key, mapErr(err))
	}
	return obj, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		if errors.Is(mapErr(err), ErrNotFound) {
			return nil
		}
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (s *S3) List(ctx context.Context, prefix string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		// The listing stops when ctx is done, so leaving the loop early
		// must cancel it.
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if obj.Err != nil {
				yield("", fmt.Errorf("list %s: %w", prefix, obj.Err))
				return
			}
			if !yield(obj.Key, nil) {
				return
			}
		}
		if err := ctx.Err(); err != nil {
			yield("", fmt.Errorf("list %s: %w", prefix, err))
		}
	}
}

func mapErr(err error) error {
	switch minio.ToErrorResponse(err).Code {
	case minio.NoSuchKey:
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	}
	return err
}
