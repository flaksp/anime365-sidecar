package anime365client

type WebError struct {
	Status     string
	StatusCode int
}

func (e WebError) Error() string {
	return "Unexpected HTTP status code: " + e.Status
}
