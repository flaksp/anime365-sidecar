package emby

import (
	"context"
	"fmt"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
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

	for translationID := range translationIDs {
		item, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
		if err != nil {
			return false, fmt.Errorf("get episode versions: %w", err)
		}

		itemIDs[item.Id] = struct{}{}
		for _, source := range item.MediaSources {
			if source.Id != "" {
				versionIDs[source.Id] = struct{}{}
			}
		}
		// An episode with only one version may not expose MediaSources.
		if len(item.MediaSources) == 0 {
			versionIDs[item.Id] = struct{}{}
		}
	}

	var lastPlayed time.Time

	for versionID := range versionIDs {
		version, err := s.embyClient.GetUserItem(ctx, s.embyUserID, versionID)
		if err != nil {
			return false, fmt.Errorf("get episode version user data: %w", err)
		}

		itemIDs[version.Id] = struct{}{}
		itemIDs[versionID] = struct{}{}

		if version.UserData != nil && version.UserData.LastPlayedDate.After(lastPlayed) {
			lastPlayed = version.UserData.LastPlayedDate
		}
	}

	if !lastPlayed.IsZero() && time.Since(lastPlayed) < deleteAfter {
		return false, nil
	}

	sessions, err := s.embyClient.GetSessions(ctx)
	if err != nil {
		return false, fmt.Errorf("get active Emby sessions: %w", err)
	}

	for _, session := range sessions {
		if session.NowPlayingItem != nil {
			if _, playing := itemIDs[session.NowPlayingItem.Id]; playing {
				return false, nil
			}
		}

		if session.PlayState != nil {
			if _, playing := itemIDs[session.PlayState.MediaSourceId]; playing {
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
