package emby

import (
	"context"
	"fmt"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/embyclient"
)

// DeleteEpisodeIfStale checks every version before deleting any of the episode's
// downloaded translations. Eligibility as watched/completed is decided by Anime365.
func (s *Service) DeleteEpisodeIfStale(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	deleteAfter time.Duration,
) (bool, error) {
	translationIDs := s.manifestService.GetTranslationIDs(showID, episodeID)
	versionIDs := make(map[string]struct{})
	itemIDs := make(map[string]struct{})
	mediaSourceIDs := make(map[string]struct{})

	for translationID := range translationIDs {
		item, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
		if err != nil {
			return false, fmt.Errorf("get episode versions: %w", err)
		}

		itemIDs[value(item.Id)] = struct{}{}

		versionIDs[value(item.Id)] = struct{}{}
		for _, source := range item.MediaSources {
			if value(source.Id) != "" {
				mediaSourceIDs[value(source.Id)] = struct{}{}
			}
			// Media source IDs identify playback streams, not library items.
			// Alternate versions expose their library item ID separately.
			if value(source.ItemId) != "" {
				versionIDs[value(source.ItemId)] = struct{}{}
			}
		}
	}

	var lastPlayed time.Time

	for versionID := range versionIDs {
		version, err := getEmbyJSONResponse[embyclient.BaseItemDto](
			s.embyClient.GetUsersByUseridItemsByIdWithResponse(ctx, s.embyUserID, versionID),
		)
		if err != nil {
			return false, fmt.Errorf("get episode version user data: %w", err)
		}

		if value(version.Id) == "" {
			return false, fmt.Errorf("Emby returned no item for %s", versionID)
		}

		itemIDs[value(version.Id)] = struct{}{}
		itemIDs[versionID] = struct{}{}

		if version.UserData != nil && value(version.UserData.LastPlayedDate).After(lastPlayed) {
			lastPlayed = value(version.UserData.LastPlayedDate)
		}
	}

	if !lastPlayed.IsZero() && time.Since(lastPlayed) < deleteAfter {
		return false, nil
	}

	sessions, err := getEmbyJSONResponse[[]embyclient.SessionSessionInfo](
		s.embyClient.GetSessionsWithResponse(ctx, nil),
	)
	if err != nil {
		return false, fmt.Errorf("get active Emby sessions: %w", err)
	}

	for _, session := range sessions {
		if session.NowPlayingItem != nil {
			if _, playing := itemIDs[value(session.NowPlayingItem.Id)]; playing {
				return false, nil
			}
		}

		if session.PlayState != nil {
			if _, playing := mediaSourceIDs[value(session.PlayState.MediaSourceId)]; playing {
				return false, nil
			}
		}
	}

	for translationID := range translationIDs {
		if err := s.DeleteTranslation(showID, episodeID, translationID); err != nil {
			return false, err
		}
	}

	return len(translationIDs) > 0, nil
}
