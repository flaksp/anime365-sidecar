package embyclient

import (
	"context"
	"net/http"
)

func (c *Client) RefreshLibrary(
	ctx context.Context,
) error {
	err := c.sendWriteRequestToAPI(ctx, http.MethodPost, "/Library/Refresh", nil, nil, nil)

	return err
}
