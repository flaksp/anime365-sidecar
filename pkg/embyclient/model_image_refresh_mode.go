/*
 * Emby Server REST API
 *
 * Explore the Emby Server API
 *
 */
package embyclient

type ImageRefreshMode string

const (
	VALIDATION_ONLY_ImageRefreshMode ImageRefreshMode = "ValidationOnly"
	DEFAULT__ImageRefreshMode        ImageRefreshMode = "Default"
	FULL_REFRESH_ImageRefreshMode    ImageRefreshMode = "FullRefresh"
)
