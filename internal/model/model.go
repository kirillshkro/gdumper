package model

// Author — автор сообщения или темы.
type Author struct {
	// Name — имя пользователя.
	Name       string `json:"name"`
	// ProfileURL — абсолютный URL профиля пользователя.
	ProfileURL string `json:"profile_url"`
}

// Topic представляет тему форума
type Topic struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Author     Author `json:"author"`
	Created    string `json:"created_at"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"pages_total"`
	Content    string `json:"content"`
	Messages   []*Message
}

// Message представляет сообщение в теме
type Message struct {
	ID          string `json:"id"`
	TopicID     string
	Author      Author `json:"author"`
	Date        string `json:"created_at"`
	BodyHTML    string `json:"body_html"`
	BodyText    string `json:"body_text"`
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
	Name  string `json:"name"`
	URL       string `json:"url"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
	LocalPath  string `json:"local_path"`
	SHA256      string `json:"sha256"`
}
