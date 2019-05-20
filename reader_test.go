package creader

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"testing"
)

type randomReaderAt struct{}

// Read read random bytes into b, always returns err = nil.
func (r *randomReaderAt) ReadAt(b []byte, offset int64) (n int, err error) {
	return rand.Read(b)
}

type nullReaderAt struct{}

// Read '0' runes into b, always return err = nil.
func (r *nullReaderAt) ReadAt(bb []byte, offset int64) (n int, err error) {
	for k := range bb {
		bb[k] = '0'
	}
	return len(bb), nil
}

type neverReaderAt struct{}

// Fulfill the interface, but never return enough bytes, just block.
func (r *neverReaderAt) ReadAt(b []byte, offset int64) (n int, err error) {
	for i := 0; i < len(b)-1; i++ {
		b[i] = '0'
		n++
	}

	ch := make(chan struct{})
	<-ch

	return n, nil
}

func BenchmarkConcurrentReader_ReadAll(b *testing.B) {
	tests := []struct {
		name    string
		workers int
	}{
		{name: "%d workers %s", workers: 1 << 0},
		{name: "%d workers %s", workers: 1 << 1},
		{name: "%d workers %s", workers: 1 << 2},
		{name: "%d workers %s", workers: 1 << 3},
		{name: "%d workers %s", workers: 1 << 4},
	}
	var list []Chunk
	for _, tt := range tests {
		for i := 0; i < 4; i++ {
			j := int64(1 << uint(10*i))
			b.Run(fmt.Sprintf(tt.name, tt.workers, ByteCountBinary(j)), func(b *testing.B) {
				cr, _ := NewConcurrentReader(&nullReaderAt{}, j)
				for n := 0; n < b.N; n++ {
					list = cr.Chop()
				}
			})
		}
	}
	_ = list
}

//go:generate dd if=/dev/random of=testdata/test.bin bs=512 count=1025

func TestConcurrentReader_Chop(t *testing.T) {
	file, err := os.Open("testdata/test.bin")
	if err != nil {
		t.Errorf("Error while loading testdata: %v", err)
	}
	defer file.Close()

	finfo, err := file.Stat()
	if err != nil {
		t.Errorf("Error while reading test file: %v", err)
	}

	// File is expected to be 512.5 KiB in size for this test.
	size := finfo.Size()
	if size != 512.5*Ki {
		t.Errorf("Test file size expected to be 512.5 KiB, actual size: %s", ByteCountBinary(size))
	}

	reader, _ := NewConcurrentReader(file, size, WithChunkSize(256))
	out := make(chan Chunk)
	var list []Chunk

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var n int64
		for ch := range out {
			m, err := io.Copy(ioutil.Discard, &ch)
			if err != nil {
				t.Error(err)
			}
			n += m
		}
		if n != 512.5*Ki {
			t.Errorf("Read %d bytes, %f bytes wanted", n, 512.5*Ki)
		}
		t.Logf("Read %d bytes", n)
	}()

	tests := []struct {
		name string
		c    *ConcurrentReader
	}{
		{name: "test 512 KiB file", c: reader},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list = tt.c.Chop()
			for _, v := range list {
				out <- v
			}
		})
	}
	close(out)
	wg.Wait()
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func TestChunk_Size(t *testing.T) {
	buf := bytes.NewReader(randBytes(1 * Mi))
	tests := []struct {
		name string
		c    *Chunk
		want int64
	}{
		{"1 KiB", &Chunk{0, Ki, io.NewSectionReader(buf, 0, 1*Ki), 0}, 1 * Ki},
		{"0.5 KiB", &Chunk{0, 0.5 * Ki, io.NewSectionReader(buf, 0, 0.5*Ki), 0}, 0.5 * Ki},
		{"0 KiB", &Chunk{0, 0, io.NewSectionReader(buf, 0, 0), 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Size(); got != tt.want {
				t.Errorf("Chunk.Size() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChunk_Hash(t *testing.T) {
	bb := randBytes(64)
	buf := bytes.NewReader(bb)
	hsha1 := sha1.New()
	hsha256 := sha256.New()
	hmd5 := md5.New()

	c := &Chunk{
		offset: 0,
		size:   int64(len(bb)),
		sr:     io.NewSectionReader(buf, 0, int64(len(bb))),
		index:  0,
	}

	hashes := make(map[string][]byte)
	bmd5 := md5.Sum(bb)
	bsha1 := sha1.Sum(bb)
	bsha256 := sha256.Sum256(bb)

	hashes["md5"] = bmd5[:]
	hashes["sha1"] = bsha1[:]
	hashes["sha256"] = bsha256[:]

	type args struct {
		h hash.Hash
	}
	tests := []struct {
		name string
		c    *Chunk
		args args
		want []byte
	}{
		{"md5", c, args{hmd5}, hashes["md5"]},
		{"sha1", c, args{hsha1}, hashes["sha1"]},
		{"sha256", c, args{hsha256}, hashes["sha256"]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.Hash(tt.args.h); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Chunk.Hash() = %v, want %v", got, tt.want)
			}
		})
	}
}
