package fs

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOSFilesystemImplementsFS(t *testing.T) {
	var fs Filesystem
	fs = OS{}
	_ = fs
}

func TestOSFindFilesSuccess(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	logdir := filepath.Join(dir, "logs")

	mkdirHard(t, filepath.Join(logdir))
	mkdirHard(t, filepath.Join(logdir, "emptydir"))
	mkdirHard(t, filepath.Join(logdir, "run1"))
	mkdirHard(t, filepath.Join(logdir, "run2"))
	mkdirHard(t, filepath.Join(logdir, "nonrun"))

	touchHard(t, filepath.Join(logdir, "run1", "tfevents.1"))
	touchHard(t, filepath.Join(logdir, "run2", "tfevents.1"))
	touchHard(t, filepath.Join(logdir, "run2", "tfevents.2"))
	touchHard(t, filepath.Join(logdir, "run2", "randomfile1"))
	touchHard(t, filepath.Join(logdir, "nonrun", "randomfile2"))

	gotFiles, err := OS{}.FindFiles(logdir, "*tfevents*")
	wantFiles := []string{
		filepath.Join(logdir, "run1", "tfevents.1"),
		filepath.Join(logdir, "run2", "tfevents.1"),
		filepath.Join(logdir, "run2", "tfevents.2"),
	}
	if err != nil || !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Errorf(`FindFiles(logdir, "*tfevents*"): got %v, %v; want %v, %v`, gotFiles, err, wantFiles, nil)
	}
}

func TestOSFindFilesBadPattern(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	logdir := filepath.Join(dir, "logs")
	mkdirHard(t, logdir)
	// We need to actually process a file for the error condition to be
	// hit; filepath doesn't offer a way to precompile the pattern.
	touchHard(t, filepath.Join(logdir, "somefile"))

	pat := "[" // ]
	files, err := OS{}.FindFiles(logdir, pat)
	if len(files) != 0 || !errors.Is(err, filepath.ErrBadPattern) {
		t.Errorf(`FindFiles(logdir, %q): got %v, %v; want nil, ErrBadPattern`, pat, files, err)
	}
}

func TestOSFindFilesOSError(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	logdir := filepath.Join(dir, "logs")
	mkdirHard(t, logdir)
	nondir := filepath.Join(logdir, "enoent")

	pat := "*tfevents*"
	files, err := OS{}.FindFiles(nondir, pat)
	if len(files) != 0 || !os.IsNotExist(err) {
		t.Errorf("FindFiles(%q, %q): got %v, %v; want nil, ENOENT", nondir, pat, files, err)
	}
}

func TestOSListFilesSuccess(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	logdir := filepath.Join(dir, "logs")
	mkdirHard(t, filepath.Join(logdir))
	mkdirHard(t, filepath.Join(logdir, "subdir"))

	touchHard(t, filepath.Join(logdir, "file1"))
	touchHard(t, filepath.Join(logdir, "file2"))
	touchHard(t, filepath.Join(logdir, "subdir", "file3"))

	gotFiles, err := OS{}.ListFiles(logdir)
	wantFiles := []string{
		filepath.Join(logdir, "file1"),
		filepath.Join(logdir, "file2"),
	}
	if err != nil || !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Errorf("ListFiles(%q): got %v, %v; want %v, %v", logdir, gotFiles, err, wantFiles, nil)
	}
}

func TestOSListFilesOSError(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	logdir := filepath.Join(dir, "logs")
	mkdirHard(t, filepath.Join(logdir))
	touchHard(t, filepath.Join(logdir, "file1"))
	nondir := filepath.Join(dir, "enoent")

	files, err := OS{}.ListFiles(nondir)
	if len(files) != 0 || !os.IsNotExist(err) {
		t.Errorf("ListFiles(%q): got %v, %v; want nil, ENOENT", nondir, files, err)
	}
}

func TestOSOpenSuccess(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	logdir := filepath.Join(dir, "logs")
	mkdirHard(t, logdir)
	path := filepath.Join(logdir, "myfile")
	{
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("os.Create(%q): %v", path, err)
		}
		if _, err := f.Write([]byte("hello")); err != nil {
			t.Fatalf("f.Write(...): %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("f.Close(): %v", err)
		}
	}

	f, err := OS{}.Open(path)
	if err != nil {
		t.Fatalf("OS{}.Open(%q): %v", path, err)
	}
	var buf bytes.Buffer
	if n, err := buf.ReadFrom(f); err != nil {
		t.Fatalf("first buf.ReadFrom(f): %v, %v", n, err)
	}
	if n, err := f.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("f.Seek(4, io.SeekStart): %v, %v", n, err)
	}
	if got, want := buf.String(), "hello"; got != want {
		t.Errorf("first buf.String(): got %q, want %q", got, want)
	}
	if n, err := buf.ReadFrom(f); err != nil {
		t.Fatalf("second buf.ReadFrom(f): %v, %v", n, err)
	}
	if got, want := buf.String(), "helloo"; got != want {
		t.Errorf("first buf.String(): got %q, want %q", got, want)
	}
	if err := f.Close(); err != nil {
		t.Errorf("f.Close(): got %v, want nil", err)
	}
}

func TestOSOpenOSError(t *testing.T) {
	dir := tempdir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "enoent")
	f, err := OS{}.Open(path)
	if !os.IsNotExist(err) {
		t.Errorf("OS{}.Open(%q): got %v, %v; want nil, ENOENT", path, f, err)
	}
}

func tempdir(t *testing.T) string {
	name, err := ioutil.TempDir("", "fs_os_test")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	return name
}

func mkdirHard(t *testing.T, name string) {
	mode := os.FileMode(0700)
	err := os.Mkdir(name, mode)
	if err != nil {
		t.Fatalf("os.Mkdir(%q, %O) = %v", name, mode, err)
	}
}

func touchHard(t *testing.T, name string) {
	f, err := os.Create(name)
	if err != nil {
		t.Fatalf("os.Create(%q) = %v, %v", name, f, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("f<%q>.Close() = %v", name, err)
	}
}
