package anime365client

import "fmt"

const (
	ErrorCodeNotFound           = 404
	ErrorAuthenticationRequired = 403
)

type APIError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Code, e.Message)
}
