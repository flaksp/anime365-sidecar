package show

import (
	"net/url"
	"strconv"
	"time"

	"github.com/flaksp/anime365-sidecar/pkg/shikimoriclient"
)

func NewShowFromShikimoriFromDTO(dto shikimoriclient.Anime) (ShowFromShikimori, error) {
	malID, err := strconv.Atoi(dto.ID)
	if err != nil {
		return ShowFromShikimori{}, err
	}

	res := ShowFromShikimori{
		ID: MyAnimeListID(malID),
	}

	if dto.Rating != nil {
		res.AgeRating = AgeRating(*dto.Rating)
	}

	if dto.Duration != nil && *dto.Duration > 0 {
		res.AverageEpisodeDuration = time.Duration(*dto.Duration) * time.Minute
	}

	if dto.AiredOn != nil && dto.AiredOn.Day != nil && *dto.AiredOn.Day > 0 && dto.AiredOn.Month != nil &&
		*dto.AiredOn.Month > 0 &&
		dto.AiredOn.Year != nil &&
		*dto.AiredOn.Year > 0 {
		res.PremiereDate = time.Date(
			*dto.AiredOn.Year,
			time.Month(*dto.AiredOn.Month),
			*dto.AiredOn.Day,
			0,
			0,
			0,
			0,
			time.UTC,
		)
	}

	if dto.Status != nil {
		switch *dto.Status {
		case shikimoriclient.AnimeStatusEnumAnons:
			res.AiringStatus = AiringStatusAnons

		case shikimoriclient.AnimeStatusEnumOngoing:
			res.AiringStatus = AiringStatusOngoing

		case shikimoriclient.AnimeStatusEnumReleased:
			res.AiringStatus = AiringStatusReleased
		}
	}

	if dto.ReleasedOn != nil && dto.ReleasedOn.Day != nil && *dto.ReleasedOn.Day > 0 && dto.ReleasedOn.Month != nil &&
		*dto.ReleasedOn.Month > 0 &&
		dto.ReleasedOn.Year != nil &&
		*dto.ReleasedOn.Year > 0 {
		res.StoppedAiringAt = time.Date(
			*dto.ReleasedOn.Year,
			time.Month(*dto.ReleasedOn.Month),
			*dto.ReleasedOn.Day,
			0,
			0,
			0,
			0,
			time.UTC,
		)
	} else if dto.Status != nil && *dto.Status == shikimoriclient.AnimeStatusEnumReleased {
		res.StoppedAiringAt = res.PremiereDate
	}

	if dto.Studios != nil {
		res.Studios = make([]string, 0, len(dto.Studios))

		for _, studio := range dto.Studios {
			res.Studios = append(res.Studios, studio.Name)
		}
	}

	if dto.Screenshots != nil {
		res.Screenshots = make([]Screenshot, 0, len(dto.Screenshots))

		for _, studio := range dto.Screenshots {
			screenshotImageURL, err := url.Parse(studio.OriginalURL)
			if err != nil {
				continue
			}

			res.Screenshots = append(res.Screenshots, Screenshot{
				ID:       studio.ID,
				ImageURL: screenshotImageURL,
			})
		}
	}

	if dto.PersonRoles != nil {
		for _, staffMember := range dto.PersonRoles {
			for _, role := range staffMember.RolesEn {
				switch role {
				case "Director":
					res.StaffMembers = append(res.StaffMembers, StaffMember{
						Name: staffMember.Person.Name,
						Role: StaffRoleDirector,
					})
				case "Producer":
					res.StaffMembers = append(res.StaffMembers, StaffMember{
						Name: staffMember.Person.Name,
						Role: StaffRoleProducer,
					})
				case "Script":
					res.StaffMembers = append(res.StaffMembers, StaffMember{
						Name: staffMember.Person.Name,
						Role: StaffRoleScript,
					})
				case "Music":
					res.StaffMembers = append(res.StaffMembers, StaffMember{
						Name: staffMember.Person.Name,
						Role: StaffRoleMusic,
					})
				}
			}
		}
	}

	return res, nil
}

type AiringStatus string

const (
	AiringStatusAnons    AiringStatus = "anons"
	AiringStatusOngoing  AiringStatus = "ongoing"
	AiringStatusReleased AiringStatus = "released"
)

type ShowFromShikimori struct {
	PremiereDate           time.Time
	StoppedAiringAt        time.Time
	AgeRating              AgeRating
	AiringStatus           AiringStatus
	Studios                []string
	Screenshots            []Screenshot
	StaffMembers           []StaffMember
	ID                     MyAnimeListID
	AverageEpisodeDuration time.Duration
}

type Screenshot struct {
	ImageURL *url.URL
	ID       string
}

type StaffRole string

const (
	StaffRoleDirector = StaffRole("Director")
	StaffRoleProducer = StaffRole("Producer")
	StaffRoleScript   = StaffRole("Script")
	StaffRoleMusic    = StaffRole("Music")
)

type StaffMember struct {
	Name string
	Role StaffRole
}
