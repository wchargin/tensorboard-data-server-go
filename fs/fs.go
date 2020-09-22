package fs

import (
	"io"
)

// Filesystem provides a select set of basic filesystem operations as a
// compatibility mechanism.
type Filesystem interface {
	// FindFiles finds all files under the given directory whose basenames
	// match the provided pattern. The pattern matching semantics are as for
	// filepath.Match. The result list does not include directories. Path
	// names in the result list include the directory prefix: i.e., they are
	// like "dirPath/subdir/file", not just "file" or "subdir/file". Path
	// names are listed in lexical order.
	FindFiles(dirPath string, basenameGlob string) ([]string, error)
	// ListFiles lists the files in a directory. The result list does not
	// include directories, nor does it recur down the directory tree. The
	// resulting paths include the directory prefix. Path names are listed
	// in lexical order.
	ListFiles(dirPath string) ([]string, error)
	// Open opens an absolute filepath for reading.
	Open(path string) (File, error)
}

// File wraps the Reader, Seeker, and Closer interfaces.
type File interface {
	io.Reader
	io.Seeker
	io.Closer
}
