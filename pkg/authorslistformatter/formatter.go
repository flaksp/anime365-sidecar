package authorslistformatter

import (
	"fmt"
	"strings"
)

// Format formats authors list. Assuming first element of the slice is a team name or main author.
// Examples: "Unknown", "Foo ft. Bar", "Foo ft. Bar and Baz", "Foo ft. Bar, Baz and Bae".
func Format(authorsList []string) string {
	if len(authorsList) == 0 {
		return "Unknown"
	}

	if len(authorsList) == 1 {
		return authorsList[0]
	}

	team := authorsList[0]

	featuringAuthors := authorsList[1:]

	var featuringStr string

	if len(featuringAuthors) <= 2 {
		featuringStr = strings.Join(featuringAuthors, " and ")
	} else {
		featuringStr = strings.Join(
			featuringAuthors[:len(featuringAuthors)-1],
			", ",
		) + " and " + featuringAuthors[len(featuringAuthors)-1]
	}

	return fmt.Sprintf("%s ft. %s", team, featuringStr)
}
