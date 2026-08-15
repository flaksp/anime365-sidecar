package embyclient

import (
	"context"
	"net/http"
	"net/url"
)

// RefreshItemImages asks Emby to discover missing artwork for an existing item.
// FullRefresh is required for both metadata and images to match Emby's "Search
// for missing metadata" action. The replacement flags and item field locks
// preserve existing images and sidecar-owned textual metadata.
func (c *Client) RefreshItemImages(ctx context.Context, itemID string) error {
	queryParams := url.Values{}
	queryParams.Set("Recursive", "false")
	queryParams.Set("MetadataRefreshMode", string(FULL_REFRESH_MetadataRefreshMode))
	queryParams.Set("ImageRefreshMode", string(FULL_REFRESH_ImageRefreshMode))
	queryParams.Set("ReplaceAllMetadata", "false")
	queryParams.Set("ReplaceAllImages", "false")

	return c.sendWriteRequestToAPI(
		ctx,
		http.MethodPost,
		"/Items/"+itemID+"/Refresh",
		queryParams,
		nil,
		nil,
	)
}
