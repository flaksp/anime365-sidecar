package scansource

import (
	"strconv"
	"strings"

	"github.com/flaksp/anime365-sidecar/internal/show"
)

type SourceList string

const (
	SourceListWatching SourceList = "list_watching"
	SourceListPlanned  SourceList = "list_planned"
)

func NewService(
	scanSources []string,
) *Service {
	sourceLists, forcedShowIDs := parseScanSources(scanSources)

	return &Service{
		sourceLists:   sourceLists,
		forcedShowIDs: forcedShowIDs,
	}
}

type Service struct {
	sourceLists   map[SourceList]struct{}
	forcedShowIDs map[show.Anime365SeriesID]struct{}
}

func (s *Service) HasList(sourceList SourceList) bool {
	_, ok := s.sourceLists[sourceList]

	return ok
}

func (s *Service) HasShow(showID show.Anime365SeriesID) bool {
	_, ok := s.forcedShowIDs[showID]

	return ok
}

func (s *Service) GetForcedShowIDs() map[show.Anime365SeriesID]struct{} {
	return s.forcedShowIDs
}

func parseScanSources(scanSources []string) (map[SourceList]struct{}, map[show.Anime365SeriesID]struct{}) {
	sourceLists := make(map[SourceList]struct{})
	forcedShowIDs := make(map[show.Anime365SeriesID]struct{})

	for _, scanSource := range scanSources {
		if strings.TrimSpace(scanSource) == string(SourceListWatching) {
			sourceLists[SourceListWatching] = struct{}{}

			continue
		}

		if strings.TrimSpace(scanSource) == string(SourceListPlanned) {
			sourceLists[SourceListPlanned] = struct{}{}

			continue
		}

		// Collecting IDs in separate map to overwrite all list entries later
		if showIDInt, err := strconv.Atoi(strings.TrimSpace(scanSource)); err == nil && showIDInt > 0 {
			forcedShowIDs[show.Anime365SeriesID(showIDInt)] = struct{}{}

			continue
		}
	}

	return sourceLists, forcedShowIDs
}
