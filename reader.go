package creader

import (
	"bytes"
	"fmt"
	"hash"
	"io"
)

// Chunk wraps an io.SectionReader with offset information and an index showing
// the position relative to other chunks.
type Chunk struct {
	offset int64
	size   int64
	sr     *io.SectionReader
	index  int
}

// Read proxies the underlying SectionReader making this an io.Reader.
func (c *Chunk) Read(p []byte) (n int, err error) {
	return c.sr.Read(p)
}

// Size returns the len of Data.
func (c *Chunk) Size() int64 {
	return c.sr.Size()
}

// Index returns the 0-based index of the chunk relative to other chunks.
func (c *Chunk) Index() int {
	return c.index
}

// Hash returns the hash of this chunk.
func (c *Chunk) Hash(h hash.Hash) []byte {
	h.Reset() // Just in case.
	io.Copy(h, c.sr)
	c.sr.Seek(0, 0)
	return h.Sum(nil)
}

// Offset returns the byte offset of the chunk.
func (c *Chunk) Offset() int64 {
	return c.offset
}

// Data reads from the embedded io.SectionReader and returns a copy of the
// []byte read. This allocates memory for the whole chunk. Be careful!
func (c *Chunk) Data() []byte {
	var buf bytes.Buffer
	io.Copy(&buf, c.sr)
	c.sr.Seek(0, 0)
	return buf.Bytes()
}

// A ConcurrentReader wraps io.ReaderAt and splits it up into smaller
// io.SectionReaders (chunks) with an index which read concurrently from the
// same underlying io.ReaderAt. Note: io.ReaderAt is goroutine-safe.
type ConcurrentReader struct {
	r  io.ReaderAt // Wrapped reader
	cs int64       // Chunk size for reads.

	Chunks []Chunk // List of byte ranges to cover.
}

// ConcurrentReaderOpt is a config opttion for the ConcurrentReader.
type ConcurrentReaderOpt func(*ConcurrentReader) error

// WithChunkSize sets the chunk size for this reader.
func WithChunkSize(n int) ConcurrentReaderOpt {
	return func(r *ConcurrentReader) error {
		if n < 1 {
			return fmt.Errorf("chunk sizes below 1 not allowed, given: %d", n)
		}
		r.cs = int64(n)
		return nil
	}
}

// NewConcurrentReader creates a concurrent reader with an embedded io.ReaderAt.
// `size` is used by ReadAll and must be set to the total size of the stream
// (usually a file) on io.ReaderAt.
func NewConcurrentReader(in io.ReaderAt, size int64, opts ...ConcurrentReaderOpt) (*ConcurrentReader, error) {
	if size < 0 {
		return nil, fmt.Errorf("size should not be negative: %d", size)
	}

	cr := &ConcurrentReader{
		r:  in,
		cs: int64(4 * 1 << 20), // 4 MiB default
	}
	for _, opt := range opts {
		if err := opt(cr); err != nil {
			return nil, err
		}
	}

	cs := cr.cs
	nchunks := int((size + cs - 1) / cs)
	chunks := make([]Chunk, nchunks)
	for i := 0; i < nchunks; i++ {
		chunks[i].offset = int64(i) * cs
		chunks[i].size = cs
		chunks[i].index = i
	}

	// Change last chunk size to the remainder of bytes not fitting into
	// chunksize (cs).
	if rem := size % cs; rem != 0 {
		chunks[len(chunks)-1].size = rem
	}

	cr.Chunks = chunks
	return cr, nil
}

// Chop chops up the embedded reader into several SectionReaders so we can read
// chunks of the underlying io.ReaderAt concurrently. The list is ordered from
// the start of the stream to the end.
func (c *ConcurrentReader) Chop() []Chunk {
	// Chop it up into section readers.
	for k, chunk := range c.Chunks {
		c.Chunks[k].sr = io.NewSectionReader(c.r, chunk.offset, chunk.size)
	}
	return c.Chunks
}

// ReadAt accesses the underlying reader directly. This is to still satisfy the
// io.ReaderAt interface and for convenience in case we want to read from the
// stream at a specific position.
func (c *ConcurrentReader) ReadAt(p []byte, off int64) (n int, err error) {
	return c.r.ReadAt(p, off)
}
