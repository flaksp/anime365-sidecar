package filesystemutils

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// CopyThenDelete solves Linux problem when it cannot rename (move) files across filesystems.
// On Windows it solves problem of copying file across disks.
func CopyThenDelete(sourceFilePath, destinationFilePath string) error {
	// Try atomic rename first (works when src and dst are on the same filesystem)
	if err := os.Rename(sourceFilePath, destinationFilePath); err == nil {
		return nil
	}

	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close() // nolint:errcheck

	destinationFile, err := os.Create(destinationFilePath)
	if err != nil {
		return err
	}

	defer func() {
		destinationFile.Close() // nolint:errcheck

		if err != nil {
			os.Remove(destinationFilePath) // nolint:errcheck
		}
	}()

	if _, err = io.Copy(destinationFile, sourceFile); err != nil {
		return err
	}

	if err := destinationFile.Sync(); err != nil {
		return err
	}

	if err := destinationFile.Close(); err != nil {
		return err
	}

	return os.Remove(sourceFilePath)
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

	if err := os.Remove(absolutePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
