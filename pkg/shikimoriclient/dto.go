package shikimoriclient

type GetAnimesResponse struct {
	Animes []Anime `json:"animes"`
}

type Anime struct {
	Rating      *AnimeRatingEnum `json:"rating"`
	AiredOn     *IncompleteDate  `json:"airedOn"`
	ReleasedOn  *IncompleteDate  `json:"releasedOn"`
	Duration    *int             `json:"duration"`
	ID          string           `json:"id"`
	Status      string           `json:"status"`
	Studios     []Studio         `json:"studios"`
	Screenshots []Screenshot     `json:"screenshots"`
	PersonRoles []PersonRole     `json:"personRoles"`
}

type IncompleteDate struct {
	Day   *int `json:"day"`
	Month *int `json:"month"`
	Year  *int `json:"year"`
}

type Studio struct {
	Name string `json:"name"`
}

type Screenshot struct {
	ID          string `json:"id"`
	OriginalURL string `json:"originalUrl"`
}

type PersonRole struct {
	Person  Person   `json:"person"`
	RolesEn []string `json:"rolesEn"`
}

type Person struct {
	Name string `json:"name"`
}

type AnimeRatingEnum string

const (
	AnimeRatingEnumNone  AnimeRatingEnum = "none"
	AnimeRatingEnumG     AnimeRatingEnum = "g"
	AnimeRatingEnumPG    AnimeRatingEnum = "pg"
	AnimeRatingEnumPG13  AnimeRatingEnum = "pg_13"
	AnimeRatingEnumR     AnimeRatingEnum = "r"
	AnimeRatingEnumRPlus AnimeRatingEnum = "r_plus"
	AnimeRatingEnumRx    AnimeRatingEnum = "rx"
)
