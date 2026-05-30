package filesystemutils

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// CopyThenDelete solves Linux problem when it cannot rename (move) files across filesystems.
func CopyThenDelete(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() // nolint:errcheck

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close() // nolint:errcheck

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	if err := out.Sync(); err != nil {
		return err
	}

	return os.Remove(src)
}

var ErrNotFile = errors.New("not a file")

// FileExists deletes file if it exists.
// Error ErrNotFile returned if target is not a file.
func FileExists(absolutePath string) (bool, error) {
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check file: %w", err)
	}

	if info.IsDir() {
		return false, ErrNotFile
	}

	return true, nil
}

func DeleteFileIfExists(absolutePath string) error {
	fileExists, err := FileExists(absolutePath)
	if err != nil {
		return fmt.Errorf("failed to check file exists: %w", err)
	}

	if !fileExists {
		return nil
	}

	if err = os.Remove(absolutePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
