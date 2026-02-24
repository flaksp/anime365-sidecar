package embyclient

import "context"

func (c *Client) GetLibraryVirtualFolders(
	ctx context.Context,
) ([]VirtualFolderInfo, error) {
	var response []VirtualFolderInfo

	err := c.sendGETRequestToAPI(ctx, "/Library/VirtualFolders", nil, &response)

	return response, err
}
