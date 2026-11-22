package episode

import (
	"slices"
	"time"
)

type CatalogMetadata struct {
	AiredAt time.Time
	Filler  *bool
	Recap   *bool
	Title   string
}

// ApplyTitleAndDate updates only the title and air date supplied by the catalog.
func (m CatalogMetadata) ApplyTitleAndDate(name **string, premiereDate **time.Time) bool {
	changed := false

	if m.Title != "" && (*name == nil || **name != m.Title) {
		*name = new(m.Title)
		changed = true
	}

	if date := m.AiredAt; !date.IsZero() && (*premiereDate == nil || !(**premiereDate).Equal(date)) {
		*premiereDate = new(date)
		changed = true
	}

	return changed
}

// ApplyTags reconciles the two catalog tags and retains unrelated tags.
func (m CatalogMetadata) ApplyTags(tags []string) []string {
	tags = applyCatalogTag(tags, "Филлер", m.Filler)

	return applyCatalogTag(tags, "Рекап", m.Recap)
}

func applyCatalogTag(tags []string, tag string, flag *bool) []string {
	if flag == nil {
		return tags
	}

	exists := slices.Contains(tags, tag)
	if *flag && !exists {
		return append(tags, tag)
	}

	if !*flag && exists {
		return slices.DeleteFunc(slices.Clone(tags), func(current string) bool { return current == tag })
	}

	return tags
}
