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

// Package store is the object storage Alpheus keeps chunks, manifests and
// ack files in, behind a small interface: an S3 implementation (minio-go)
// and an in-memory one for tests (GO-12).
package store

import (
	"context"
	"errors"
	"io"
	"iter"
)

// ErrNotFound means the object does not exist.
var ErrNotFound = errors.New("object not found")

// Store is a bucket of objects. Writes of one key are atomic: a reader sees
// the old or the new object, never a mix. The store must have strong
// read-after-write consistency, also for overwrites and listing (GO-69).
type Store interface {
	// Put writes the object, replacing any object with the same key. size
	// is the length of r, or -1 when it is not known in advance.
	Put(ctx context.Context, key string, r io.Reader, size int64) error

	// Get opens the object for reading. It returns ErrNotFound when the
	// object does not exist. The caller closes the reader.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object. Deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error

	// List yields the keys under prefix in lexical order.
	List(ctx context.Context, prefix string) iter.Seq2[string, error]
}

// ReadAll reads a whole, small object such as a manifest or ack file.
func ReadAll(ctx context.Context, s Store, key string) ([]byte, error) {
	r, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// DeletePrefix removes every object under prefix, for example a run's
// runs/<uid>/ when it is cleaned up (GO-20).
func DeletePrefix(ctx context.Context, s Store, prefix string) error {
	for key, err := range s.List(ctx, prefix) {
		if err != nil {
			return err
		}
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}
