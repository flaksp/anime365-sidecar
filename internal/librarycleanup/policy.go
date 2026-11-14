package librarycleanup

import (
	"fmt"
	"time"
)

type Policy string

const (
	DoNotDelete                    Policy = "do_not_delete"
	DeleteWatchedEpisodes          Policy = "delete_watched_episodes"
	DeleteCompletedAndDroppedShows Policy = "delete_completed_and_dropped_shows"
)

func (p *Policy) UnmarshalText(text []byte) error {
	policy := Policy(text)
	switch policy {
	case DoNotDelete, DeleteWatchedEpisodes, DeleteCompletedAndDroppedShows:
		*p = policy

		return nil
	default:
		return fmt.Errorf("unknown watched episodes cleanup policy %q", policy)
	}
}

type Options struct {
	Policy                    Policy
	DeleteAfter               time.Duration
	DeleteRemovedTranslations bool
}
