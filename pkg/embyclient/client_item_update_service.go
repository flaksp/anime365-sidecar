package embyclient

import (
	"context"
	"net/http"
)

func (c *Client) UpdateItem(
	ctx context.Context,
	itemID string,
	itemDTO BaseItemDto,
) error {
	return c.sendWriteRequestToAPI(ctx, http.MethodPost, "/Items/"+itemID, nil, itemDTO, nil)
}
