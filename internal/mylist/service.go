package mylist

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/flaksp/anime365-emby/internal/show"
	"github.com/flaksp/anime365-emby/pkg/anime365client"
	"golang.org/x/sync/errgroup"
)

func NewService(anime365Client *anime365client.Client, logger *slog.Logger) *Service {
	return &Service{
		anime365Client: anime365Client,
		logger:         logger,
	}
}

type Service struct {
	anime365Client *anime365client.Client
	plannedList    map[show.Anime365SeriesID]int64
	watchingList   map[show.Anime365SeriesID]int64
	droppedList    map[show.Anime365SeriesID]int64
	completedList  map[show.Anime365SeriesID]int64
	onHoldList     map[show.Anime365SeriesID]int64
	logger         *slog.Logger
	mu             sync.RWMutex
}

func (s *Service) LoadFromAnime365(ctx context.Context, profileID int64) error {
	var (
		plannedList   map[show.Anime365SeriesID]int64
		watchingList  map[show.Anime365SeriesID]int64
		droppedList   map[show.Anime365SeriesID]int64
		completedList map[show.Anime365SeriesID]int64
		onHoldList    map[show.Anime365SeriesID]int64
	)

	errGroup, errGroupCtx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		animeListItems, err := s.anime365Client.GetAnimeList(errGroupCtx, profileID, anime365client.AnimeListIDPlanned)
		if err != nil {
			return fmt.Errorf("failed to get planned list items: %w", err)
		}

		list := make(map[show.Anime365SeriesID]int64, len(animeListItems))
		for _, animeListItem := range animeListItems {
			list[show.Anime365SeriesID(animeListItem.ID)] = animeListItem.EpisodesWatched
		}

		plannedList = list

		return nil
	})

	errGroup.Go(func() error {
		animeListItems, err := s.anime365Client.GetAnimeList(errGroupCtx, profileID, anime365client.AnimeListIDWatching)
		if err != nil {
			return fmt.Errorf("failed to get watching list items: %w", err)
		}

		list := make(map[show.Anime365SeriesID]int64, len(animeListItems))
		for _, animeListItem := range animeListItems {
			list[show.Anime365SeriesID(animeListItem.ID)] = animeListItem.EpisodesWatched
		}

		watchingList = list

		return nil
	})

	errGroup.Go(func() error {
		animeListItems, err := s.anime365Client.GetAnimeList(errGroupCtx, profileID, anime365client.AnimeListIDDropped)
		if err != nil {
			return fmt.Errorf("failed to get dropped list items: %w", err)
		}

		list := make(map[show.Anime365SeriesID]int64, len(animeListItems))
		for _, animeListItem := range animeListItems {
			list[show.Anime365SeriesID(animeListItem.ID)] = animeListItem.EpisodesWatched
		}

		droppedList = list

		return nil
	})

	errGroup.Go(func() error {
		animeListItems, err := s.anime365Client.GetAnimeList(
			errGroupCtx,
			profileID,
			anime365client.AnimeListIDCompleted,
		)
		if err != nil {
			return fmt.Errorf("failed to get completed list items: %w", err)
		}

		list := make(map[show.Anime365SeriesID]int64, len(animeListItems))
		for _, animeListItem := range animeListItems {
			list[show.Anime365SeriesID(animeListItem.ID)] = animeListItem.EpisodesWatched
		}

		completedList = list

		return nil
	})

	errGroup.Go(func() error {
		animeListItems, err := s.anime365Client.GetAnimeList(errGroupCtx, profileID, anime365client.AnimeListIDOnHold)
		if err != nil {
			return fmt.Errorf("failed to get on hold list items: %w", err)
		}

		list := make(map[show.Anime365SeriesID]int64, len(animeListItems))
		for _, animeListItem := range animeListItems {
			list[show.Anime365SeriesID(animeListItem.ID)] = animeListItem.EpisodesWatched
		}

		onHoldList = list

		return nil
	})

	if err := errGroup.Wait(); err != nil {
		return err
	}

	s.mu.Lock()
	s.plannedList = plannedList
	s.watchingList = watchingList
	s.droppedList = droppedList
	s.completedList = completedList
	s.onHoldList = onHoldList
	s.mu.Unlock()

	s.logger.InfoContext(ctx,
		"Successfully loaded anime lists from Anime 365",
		slog.Int("planned", len(plannedList)),
		slog.Int("watching", len(watchingList)),
		slog.Int("dropped", len(droppedList)),
		slog.Int("completed", len(completedList)),
		slog.Int("onHold", len(onHoldList)),
	)

	return nil
}

func (s *Service) GetLastWatchedEpisodeNumber(showID show.Anime365SeriesID) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if episodeNumber, ok := s.plannedList[showID]; ok {
		return episodeNumber
	}

	if episodeNumber, ok := s.watchingList[showID]; ok {
		return episodeNumber
	}

	if episodeNumber, ok := s.droppedList[showID]; ok {
		return episodeNumber
	}

	if episodeNumber, ok := s.completedList[showID]; ok {
		return episodeNumber
	}

	if episodeNumber, ok := s.onHoldList[showID]; ok {
		return episodeNumber
	}

	return 0
}

func (s *Service) GetWatchingList() map[show.Anime365SeriesID]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.watchingList
}

func (s *Service) GetPlannedList() map[show.Anime365SeriesID]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.plannedList
}
