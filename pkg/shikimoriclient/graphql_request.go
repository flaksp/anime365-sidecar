package shikimoriclient

type graphQLRequest struct {
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
}
