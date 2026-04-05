package filesystemutils

import (
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
