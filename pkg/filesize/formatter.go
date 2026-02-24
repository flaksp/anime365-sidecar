package filesize

import "fmt"

func Format(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div := int64(unit)
	exp := 0

	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	suffixes := []string{"KB", "MB", "GB", "TB", "PB", "EB"}

	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), suffixes[exp])
}
