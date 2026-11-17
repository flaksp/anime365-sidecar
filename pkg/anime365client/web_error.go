package anime365client

type WebError struct {
	Status string
}

func (e WebError) Error() string {
	return "Unexpected HTTP status code: " + e.Status
}
