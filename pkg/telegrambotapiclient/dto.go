package telegrambotapiclient

type LinkPreviewOptions struct {
	IsDisabled *bool `json:"is_disabled"`
}

type Message struct {
	MessageID int64 `json:"message_id"`
}
