package jikanclient

type AnimeEpisode struct {
	Aired    *string `json:"aired"`
	Synopsis *string `json:"synopsis"`
	Title    string  `json:"title"`
	Filler   bool    `json:"filler"`
	Recap    bool    `json:"recap"`
}

type AnimeEpisodeListingItem struct {
	Score *float64 `json:"score"`
	MalID int64    `json:"mal_id"`
}
