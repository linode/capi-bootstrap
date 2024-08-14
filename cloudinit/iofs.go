package cloudinit

import (
	"io"
	"io/fs"
	"os"
)

// IoFS implements the FS interface but only returns a single file from stdin.
type IoFS struct {
	Reader io.Reader
}

// Open returns a File containing the contents of stdin.
func (iofs IoFS) Open(_ string) (fs.File, error) {
	return IoFile{contents: iofs.Reader}, nil
}

// IoFile implements the File interface but only returns a content from stdin.
type IoFile struct {
	contents io.Reader
}

// Stat returns the file information for [os.Stdin].
func (IoFile) Stat() (fs.FileInfo, error) {
	return os.Stdin.Stat()
}

// Read returns the contents of [os.Stdin].
func (f IoFile) Read(bytes []byte) (int, error) {
	return f.contents.Read(bytes)
}

// Close implements the [io/fs.File] interface, but is otherwise a no-op.
func (IoFile) Close() error {
	return nil
}
