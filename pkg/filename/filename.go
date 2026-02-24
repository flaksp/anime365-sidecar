package filename

import "strings"

func Clean(filename string) string {
	replacer := strings.NewReplacer(":", " ", "/", " ", "\\", " ")

	return replacer.Replace(filename)
}
