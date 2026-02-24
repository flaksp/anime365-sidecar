package episode

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flaksp/anime365-emby/pkg/anime365client"
	"golang.org/x/text/language"
)

type (
	Anime365EpisodeID     int64
	Anime365TranslationID int64
)

type Episode struct {
	FirstUploadedAt time.Time
	EpisodeLabel    string
	Translations    []Translation
	Anime365ID      Anime365EpisodeID
	EpisodeNumber   int64
	IsTrailer       bool
	IsSpecial       bool
}

const (
	TranslationKindSubtitles = TranslationKind("subtitles")
	TranslationKindDub       = TranslationKind("dub")
	TranslationKindRaw       = TranslationKind("raw")
)

type TranslationKind string

func (t TranslationKind) Label() string {
	switch t {
	case TranslationKindSubtitles:
		return "subtitles"
	case TranslationKindDub:
		return "dub"
	case TranslationKindRaw:
		return "raw"
	default:
		return "unknown"
	}
}

func (t TranslationKind) IsSubtitles() bool {
	return t == TranslationKindSubtitles
}

func (t TranslationKind) IsDub() bool {
	return t == TranslationKindDub
}

func (t TranslationKind) IsRaw() bool {
	return t == TranslationKindRaw
}

type TranslationVariant struct {
	Language language.Tag
	Kind     TranslationKind
}

type Translation struct {
	Variant          TranslationVariant
	MarkedAsActiveAt time.Time
	Authors          []string
	Anime365ID       Anime365TranslationID
	Anime365Priority int
}

func NewTranslation(translationDTO anime365client.Translation) (Translation, error) {
	translationEntity := Translation{
		Anime365ID:       Anime365TranslationID(translationDTO.ID),
		Authors:          translationDTO.AuthorsList,
		Anime365Priority: translationDTO.Priority,
	}

	switch translationDTO.TypeKind {
	case anime365client.TranslationKindSub:
		translationEntity.Variant.Kind = TranslationKindSubtitles
	case anime365client.TranslationKindVoice:
		translationEntity.Variant.Kind = TranslationKindDub
	case anime365client.TranslationKindRaw:
		translationEntity.Variant.Kind = TranslationKindRaw
	default:
		return Translation{}, ErrNormalizingEpisodeEntity
	}

	translationLanguage, err := language.Parse(translationDTO.TypeLang)
	if err != nil {
		return Translation{}, ErrNormalizingEpisodeEntity
	}

	if !anime365client.IsEmptyDateString(translationDTO.ActiveDateTime) {
		date, err := anime365client.ParseDateString(translationDTO.ActiveDateTime)
		if err != nil {
			return Translation{}, ErrNormalizingEpisodeEntity
		}

		translationEntity.MarkedAsActiveAt = date
	}

	translationEntity.Variant.Language = translationLanguage

	return translationEntity, nil
}

var ErrNormalizingEpisodeEntity = errors.New("normalizing episode entity")

func NewEpisode(episodeDTO anime365client.Episode) (Episode, error) {
	episodeEntity := Episode{
		Anime365ID:   Anime365EpisodeID(episodeDTO.ID),
		IsTrailer:    episodeDTO.EpisodeType == "preview",
		EpisodeLabel: strings.TrimSpace(episodeDTO.EpisodeFull),
	}

	episodeNumber, err := strconv.ParseInt(episodeDTO.EpisodeInt, 10, 64)
	if err != nil || episodeNumber <= 0 {
		episodeEntity.IsSpecial = true
	} else {
		episodeEntity.EpisodeNumber = episodeNumber
	}

	episodeEntity.Translations = make([]Translation, 0, len(episodeDTO.Translations))

	for _, translationDTO := range episodeDTO.Translations {
		translationEntity, err := NewTranslation(translationDTO)
		if err != nil {
			continue
		}

		episodeEntity.Translations = append(episodeEntity.Translations, translationEntity)
	}

	if !anime365client.IsEmptyDateString(episodeDTO.FirstUploadedDateTime) {
		firstUploadedAt, err := anime365client.ParseDateString(episodeDTO.FirstUploadedDateTime)
		if err != nil {
			return Episode{}, ErrNormalizingEpisodeEntity
		}

		episodeEntity.FirstUploadedAt = firstUploadedAt
	}

	return episodeEntity, nil
}

type TranslationMedia struct {
	VideoURL     *url.URL
	SubtitlesURL *url.URL
	Height       int64
}

var ErrNormalizingTranslationMediaEntity = errors.New("normalizing translation media entity")

func NewTranslationMedia(
	translationEmbedDTO anime365client.TranslationEmbed,
	anime365BaseURL *url.URL,
) (TranslationMedia, error) {
	translationMediaEntity := TranslationMedia{}

	if translationEmbedDTO.SubtitlesURL != "" {
		subtitlesURL, err := url.Parse(anime365BaseURL.String() + translationEmbedDTO.SubtitlesURL)
		if err != nil {
			return TranslationMedia{}, ErrNormalizingTranslationMediaEntity
		}

		translationMediaEntity.SubtitlesURL = subtitlesURL
	}

	for _, streamDTO := range translationEmbedDTO.Stream {
		if streamDTO.Height > translationMediaEntity.Height && len(translationEmbedDTO.Stream) > 0 {
			videoURL, err := url.Parse(streamDTO.URLs[0])
			if err != nil {
				continue
			}

			translationMediaEntity.Height = streamDTO.Height
			translationMediaEntity.VideoURL = videoURL
		}
	}

	if translationMediaEntity.VideoURL == nil {
		return TranslationMedia{}, ErrNormalizingTranslationMediaEntity
	}

	return translationMediaEntity, nil
}
