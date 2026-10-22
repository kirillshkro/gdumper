package model

// Topic представляет тему форума
type Topic struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Author     string `json:"author"`
	Created    string `json:"created"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
	Content    string `json:"content"`
	Messages   []*Message
}

// Message представляет сообщение в теме
type Message struct {
	ID          string `json:"id"`
	TopicID     string
	Author      string `json:"author"`
	Date        string `json:"date"`
	Content     string `json:"content"`
	Link        string `json:"link"`
	PostNumber  int    `json:"post_number"`
	ParentID    string `json:"parent_id"`
	Level       int    `json:"level"`
	Deleted     bool   `json:"deleted,omitempty"`
	Attachments []*Attachment `json:"attachments"`
}

// Attachment представляет вложение
type Attachment struct {
	TopicID   string
	MessageID string
	FileName  string `json:"file_name"`
	URL       string `json:"url"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
	FilePath  string `json:"file_path"`
	Hash      string `json:"hash"`
}
