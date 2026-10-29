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

package chunk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
)

// ErrChecksum means a chunk object's content does not match its manifest
// entry.
var ErrChecksum = errors.New("checksum mismatch")

// HashingWriter passes writes on and computes the SHA-256 and size of
// everything written, so a chunk's checksum is known when it is written
// (GO-31).
type HashingWriter struct {
	w io.Writer
	h hash.Hash
	n int64
}

// NewHashingWriter wraps w.
func NewHashingWriter(w io.Writer) *HashingWriter {
	return &HashingWriter{w: w, h: sha256.New()}
}

func (hw *HashingWriter) Write(p []byte) (int, error) {
	n, err := hw.w.Write(p)
	hw.h.Write(p[:n])
	hw.n += int64(n)
	return n, err
}

// Sum is the hex SHA-256 of everything written so far.
func (hw *HashingWriter) Sum() string { return hex.EncodeToString(hw.h.Sum(nil)) }

// Size is the number of bytes written so far.
func (hw *HashingWriter) Size() int64 { return hw.n }

// verifyingReader checks content against a manifest entry while it is read.
type verifyingReader struct {
	r    io.Reader
	h    hash.Hash
	n    int64
	want Chunk
}

// NewVerifyingReader wraps r, the content of chunk c. At the end of r it
// returns ErrChecksum instead of io.EOF when the size or SHA-256 differ
// from c, so a reader that reads to the end never uses a corrupt chunk
// without noticing (GO-31).
func NewVerifyingReader(r io.Reader, c Chunk) io.Reader {
	return &verifyingReader{r: r, h: sha256.New(), want: c}
}

func (vr *verifyingReader) Read(p []byte) (int, error) {
	n, err := vr.r.Read(p)
	vr.h.Write(p[:n])
	vr.n += int64(n)
	if vr.n > vr.want.Bytes {
		return n, fmt.Errorf("chunk %d: %w: more than %d bytes", vr.want.N, ErrChecksum, vr.want.Bytes)
	}
	if errors.Is(err, io.EOF) {
		if vr.n != vr.want.Bytes {
			return n, fmt.Errorf("chunk %d: %w: %d bytes, want %d", vr.want.N, ErrChecksum, vr.n, vr.want.Bytes)
		}
		if got := hex.EncodeToString(vr.h.Sum(nil)); got != vr.want.SHA256 {
			return n, fmt.Errorf("chunk %d: %w: sha256 %s, want %s", vr.want.N, ErrChecksum, got, vr.want.SHA256)
		}
	}
	return n, err
}
