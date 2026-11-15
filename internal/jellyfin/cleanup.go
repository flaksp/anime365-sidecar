package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flaksp/anime365-sidecar/internal/episode"
	"github.com/flaksp/anime365-sidecar/internal/show"
	"github.com/flaksp/anime365-sidecar/pkg/jellyfinclient"
	"github.com/google/uuid"
)

func (s *Service) DeleteTranslationIfNotPlaying(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	translationID episode.Anime365TranslationID,
) (bool, error) {
	item, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
	if err != nil {
		return false, err
	}

	playing, err := s.isPlaying(ctx, []jellyfinclient.BaseItemDto{item})
	if err != nil || playing {
		return false, err
	}

	if err := s.DeleteTranslation(showID, episodeID, translationID); err != nil {
		return false, err
	}

	return true, nil
}

// DeleteEpisodeIfStale checks user data and playback for all translations before
// removing any files. Unknown items or failed API calls prevent deletion.
func (s *Service) DeleteEpisodeIfStale(
	ctx context.Context,
	showID show.Anime365SeriesID,
	episodeID episode.Anime365EpisodeID,
	deleteAfter time.Duration,
) (bool, error) {
	translations := s.manifestService.GetTranslationIDs(showID, episodeID)
	ids := make(map[uuid.UUID]struct{})

	for translationID := range translations {
		item, err := s.getEpisodeItem(ctx, showID, episodeID, translationID)
		if err != nil {
			return false, err
		}

		id := value(item.Id)
		if id == uuid.Nil {
			return false, fmt.Errorf("Jellyfin returned an empty item ID for translation %d", translationID)
		}

		ids[id] = struct{}{}
	}

	if len(ids) == 0 {
		return false, nil
	}

	var (
		items      []jellyfinclient.BaseItemDto
		lastPlayed time.Time
	)

	for id := range ids {
		versions, err := s.getItems(ctx, jellyfinclient.GetItemsParams{Ids: []uuid.UUID{id}})
		if err != nil {
			return false, err
		}

		if len(versions) != 1 || value(versions[0].Id) != id {
			return false, fmt.Errorf("Jellyfin returned no user data item for %s", id)
		}

		version := versions[0]
		if version.UserData == nil {
			return false, fmt.Errorf("Jellyfin returned no user data for %s", id)
		}

		if played := value(version.UserData.LastPlayedDate); played.After(lastPlayed) {
			lastPlayed = played
		}

		items = append(items, version)
	}

	if !lastPlayed.IsZero() && time.Since(lastPlayed) < deleteAfter {
		return false, nil
	}

	playing, err := s.isPlaying(ctx, items)
	if err != nil || playing {
		return false, err
	}

	for translationID := range translations {
		if err := s.DeleteTranslation(showID, episodeID, translationID); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (s *Service) isPlaying(ctx context.Context, items []jellyfinclient.BaseItemDto) (bool, error) {
	itemIDs := make(map[uuid.UUID]struct{})
	sourceIDs := make(map[string]struct{})

	for _, item := range items {
		id := value(item.Id)
		if id == uuid.Nil {
			return false, errors.New("Jellyfin returned an empty playback item ID")
		}

		itemIDs[id] = struct{}{}

		for _, source := range item.MediaSources {
			if value(source.Id) != "" {
				sourceIDs[*source.Id] = struct{}{}
			}
		}
	}

	sessions, err := getJellyfinJSONResponse[[]jellyfinclient.SessionInfoDto](
		s.jellyfinClient.GetSessionsWithResponse(ctx, nil),
	)
	if err != nil {
		return false, err
	}

	for _, session := range sessions {
		if session.NowPlayingItem != nil {
			if _, playing := itemIDs[value(session.NowPlayingItem.Id)]; playing {
				return true, nil
			}
		}

		if session.PlayState != nil {
			if _, playing := sourceIDs[value(session.PlayState.MediaSourceId)]; playing {
				return true, nil
			}
		}
	}

	return false, nil
}
