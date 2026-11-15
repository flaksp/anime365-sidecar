package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// Validate selects exactly one media server and validates its conditional settings.
func (e *Env) Validate() error {
	if e.WatchedEpisodesCleanupAfter < 0 {
		return errors.New("SIDECAR_WATCHED_EPISODES_CLEANUP_AFTER must not be negative")
	}

	embyConfigured := e.EmbyBaseURL != nil || e.EmbyPublicBaseURL != nil || e.EmbyAPIKey != "" ||
		e.EmbyLibraryID != "" ||
		e.EmbyUserID != ""

	jellyfinConfigured := e.JellyfinBaseURL != nil || e.JellyfinPublicBaseURL != nil || e.JellyfinAPIKey != "" ||
		e.JellyfinLibraryID != "" ||
		e.JellyfinUserID != ""
	if embyConfigured && jellyfinConfigured {
		return errors.New("configure only one media server: Emby and Jellyfin settings cannot be combined")
	}

	var (
		prefix, key, libraryID, userID string
		baseURL, publicURL             *url.URL
	)

	switch e.MediaServer {
	case "emby":
		if jellyfinConfigured {
			return errors.New("SIDECAR_MEDIA_SERVER=emby cannot use SIDECAR_JELLYFIN_* settings")
		}

		prefix, key, libraryID, userID, baseURL, publicURL = "SIDECAR_EMBY_", e.EmbyAPIKey, e.EmbyLibraryID, e.EmbyUserID, e.EmbyBaseURL, e.EmbyPublicBaseURL
	case "jellyfin":
		if embyConfigured {
			return errors.New("SIDECAR_MEDIA_SERVER=jellyfin cannot use SIDECAR_EMBY_* settings")
		}

		prefix, key, libraryID, userID, baseURL, publicURL = "SIDECAR_JELLYFIN_", e.JellyfinAPIKey, e.JellyfinLibraryID, e.JellyfinUserID, e.JellyfinBaseURL, e.JellyfinPublicBaseURL
	default:
		return errors.New("SIDECAR_MEDIA_SERVER must be emby or jellyfin")
	}

	if err := validateURL(prefix+"BASE_URL", baseURL); err != nil {
		return err
	}

	if publicURL != nil {
		if err := validateURL(prefix+"PUBLIC_BASE_URL", publicURL); err != nil {
			return err
		}
	}

	for _, setting := range []struct{ name, value string }{{"API_KEY", key}, {"LIBRARY_ID", libraryID}, {"USER_ID", userID}} {
		if strings.TrimSpace(setting.value) == "" {
			return fmt.Errorf("%s%s is required for %s", prefix, setting.name, e.MediaServer)
		}
	}

	if e.MediaServer == "jellyfin" {
		for _, setting := range []struct{ name, value string }{{"LIBRARY_ID", libraryID}, {"USER_ID", userID}} {
			id, err := uuid.Parse(setting.value)
			if err != nil || id == uuid.Nil {
				return fmt.Errorf("%s%s must be a nonzero UUID", prefix, setting.name)
			}
		}
	}

	return nil
}

func validateURL(name string, value *url.URL) error {
	if value == nil || value.Host == "" || (value.Scheme != "http" && value.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute HTTP or HTTPS URL", name)
	}

	return nil
}
