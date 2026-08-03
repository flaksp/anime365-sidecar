package embyclient

import "context"

func (c *Client) GetSessions(ctx context.Context) ([]SessionSessionInfo, error) {
	var response []SessionSessionInfo

	err := c.sendGETRequestToAPI(ctx, "/Sessions", nil, &response)

	return response, err
}
