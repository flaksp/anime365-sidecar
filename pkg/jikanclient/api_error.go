package jikanclient

import "fmt"

const (
	ErrorCodeNotFound = 404
)

type APIError struct {
	ErrorText string `json:"error"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Status    int    `json:"status"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.Status, e.Message)
}
