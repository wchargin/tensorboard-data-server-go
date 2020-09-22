package fs

import (
	"os"
	"path/filepath"
	"sort"
)

// OS implements the Filesystem interface for the native filesystem, as with the
// standard library's "os" module and friends.
type OS struct{}

// FindFiles implements Filesystem.FindFiles.
func (OS) FindFiles(dirPath string, basenameGlob string) ([]string, error) {
	var results []string
	// Use Walk rather than Glob since Glob doesn't support escaping on
	// Windows and thus we can't safely embed the dirPath into a pattern.
	err := filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		matched, err := filepath.Match(basenameGlob, info.Name())
		if err != nil {
			return err
		}
		if matched {
			results = append(results, filePath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ListFiles implements Filesystem.ListFiles.
func (OS) ListFiles(dirPath string) ([]string, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	infos, err := f.Readdir(0)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, info := range infos {
		if !info.IsDir() {
			result = append(result, filepath.Join(dirPath, info.Name()))
		}
	}
	sort.Strings(result)
	return result, nil
}

// Open implements Filesystem.Open.
func (OS) Open(path string) (File, error) {
	return os.Open(path)
}
