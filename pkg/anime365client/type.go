package anime365client

import "github.com/flaksp/anime365-sidecar/pkg/anime365client/internal/api"

type (
	Series           = api.Series
	Episode          = api.Episode
	Translation      = api.Translation
	TranslationEmbed = api.TranslationEmbed
)

const (
	TranslationKindSub   = "sub"
	TranslationKindVoice = "voice"
	TranslationKindRaw   = "raw"
)

type Profile struct {
	Name string
	ID   int64
}

type AnimeListItem struct {
	ID              int64
	EpisodesWatched int64
}
