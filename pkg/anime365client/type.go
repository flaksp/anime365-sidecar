package anime365client

type Series struct {
	Titles struct {
		Romaji string `json:"romaji"`
		Ru     string `json:"ru"`
	} `json:"titles"`
	MyAnimeListScore string `json:"myAnimeListScore"`
	PosterURL        string `json:"posterUrl"`
	Episodes         []struct {
		EpisodeInt  string `json:"episodeInt"`
		EpisodeType string `json:"episodeType"`
		ID          int64  `json:"id"`
	} `json:"episodes"`
	Descriptions []struct {
		Source string `json:"source"`
		Value  string `json:"value"`
	} `json:"descriptions"`
	Genres []struct {
		Title string `json:"title"`
	} `json:"genres"`
	Links []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	ID            int64 `json:"id"`
	Year          int   `json:"year"`
	IsAiring      int   `json:"isAiring"`
	MyAnimeListID int64 `json:"myAnimeListId"`
}

const (
	TranslationKindSub   = "sub"
	TranslationKindVoice = "voice"
	TranslationKindRaw   = "raw"
)

type Episode struct {
	EpisodeFull           string        `json:"episodeFull"`
	EpisodeInt            string        `json:"episodeInt"`
	EpisodeType           string        `json:"episodeType"`
	FirstUploadedDateTime string        `json:"firstUploadedDateTime"`
	Translations          []Translation `json:"translations"`
	ID                    int64         `json:"id"`
}

type Translation struct {
	AuthorsSummary string   `json:"authorsSummary"`
	TypeKind       string   `json:"typeKind"`
	TypeLang       string   `json:"typeLang"`
	ActiveDateTime string   `json:"activeDateTime"`
	AuthorsList    []string `json:"authorsList"`
	ID             int64    `json:"id"`
	IsActive       int      `json:"isActive"`
	Priority       int      `json:"priority"`
}

type TranslationEmbed struct {
	SubtitlesURL string `json:"subtitlesUrl"`
	Stream       []struct {
		URLs   []string `json:"urls"`
		Height int64    `json:"height"`
	} `json:"stream"`
}

type Profile struct {
	Name string
	ID   int64
}

type AnimeListItem struct {
	ID              int64
	EpisodesWatched int64
}
