package dump

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// newWriter opens the dump output file. When compress is true the returned
// writer transparently gzip-compresses; closing it flushes the gzip stream and
// then the file.
func newWriter(dir string, set optionSet, compress bool) (io.WriteCloser, string, error) {
	name := fileName(set, compress)
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		return nil, name, fmt.Errorf("create %s: %w", path, err)
	}
	if !compress {
		return f, name, nil
	}
	return &gzipWriteCloser{gz: gzip.NewWriter(f), f: f}, name, nil
}

// gzipWriteCloser wraps a gzip.Writer over a file and closes both in order.
type gzipWriteCloser struct {
	gz *gzip.Writer
	f  *os.File
}

func (w *gzipWriteCloser) Write(p []byte) (int, error) { return w.gz.Write(p) }

func (w *gzipWriteCloser) Close() error {
	gzErr := w.gz.Close()
	fErr := w.f.Close()
	if gzErr != nil {
		return gzErr
	}
	return fErr
}
