package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kirillshkro/gdumper/internal/parser"
)

// TestParseAttachmentsXenForo — ATT-04: XenForo пост с вложением; имя,
// абсолютный URL, MIME, размер и MessageID извлекаются.
func TestParseAttachmentsXenForo(t *testing.T) {
	post := postSpec{
		id:     "100",
		author: "alice",
		date:   "2023-01-01T12:00:00Z",
		body:   "Вложение ниже.",
		number: 1,
		attachments: []attachmentSpec{
			{name: "отчёт.pdf", url: "/attachments/report.pdf", mime: "application/pdf", size: "2048"},
		},
	}
	page, err := parser.NewXenForo().ParsePage([]byte(xenForoPageHTML([]postSpec{post})), xenForoTestURL)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, page.Messages, 1)

	msg := page.Messages[0]
	require.Len(t, msg.Attachments, 1, "вложение должно быть извлечено")
	att := msg.Attachments[0]
	assert.Equal(t, "отчёт.pdf", att.Name, "имя из текста ссылки")
	assert.Equal(t, "https://forum.example.com/attachments/report.pdf", att.URL, "относительный URL должен стать абсолютным")
	assert.Equal(t, "application/pdf", att.MIME, "MIME берётся из data-mime")
	assert.Equal(t, int64(2048), att.Size, "размер берётся из data-size")
	assert.Equal(t, "100", att.MessageID, "MessageID равен ID поста")
}

// TestParseAttachmentsPHPBB — ATT-05: phpBB пост с вложением; имя, абсолютный
// относительный URL без ведущего слэша, MIME и размер извлекаются.
func TestParseAttachmentsPHPBB(t *testing.T) {
	post := postSpec{
		id:     "200",
		author: "bob",
		date:   "2023-01-01T13:00:00Z",
		body:   "Фото.",
		number: 2,
		attachments: []attachmentSpec{
			{name: "photo.jpg", url: "attachments/photo.jpg", mime: "image/jpeg", size: "4096"},
		},
	}
	page, err := parser.NewPHPBB().ParsePage([]byte(phpbbPageHTML([]postSpec{post})), phpbbTestURL)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, page.Messages, 1)

	msg := page.Messages[0]
	require.Len(t, msg.Attachments, 1, "вложение должно быть извлечено")
	att := msg.Attachments[0]
	assert.Equal(t, "photo.jpg", att.Name, "имя из текста ссылки")
	assert.Equal(t, "https://forum.example.com/attachments/photo.jpg", att.URL, "относительный URL должен стать абсолютным")
	assert.Equal(t, "image/jpeg", att.MIME, "MIME берётся из data-mime")
	assert.Equal(t, int64(4096), att.Size, "размер берётся из data-size")
	assert.Equal(t, "200", att.MessageID, "MessageID равен ID поста")
}

// TestParseAttachmentNameFallbackURL — ATT-06: вложение без текста/alt — имя берётся
// из базового имени URL, MIME определяется по расширению.
func TestParseAttachmentNameFallbackURL(t *testing.T) {
	post := postSpec{
		id:     "300",
		author: "carol",
		date:   "2023-01-01T14:00:00Z",
		body:   "",
		number: 3,
		attachments: []attachmentSpec{
			{name: "", url: "/downloads/file.bin"},
		},
	}
	page, err := parser.NewXenForo().ParsePage([]byte(xenForoPageHTML([]postSpec{post})), xenForoTestURL)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	require.Len(t, page.Messages[0].Attachments, 1, "вложение должно быть извлечено")
	att := page.Messages[0].Attachments[0]
	assert.Equal(t, "file.bin", att.Name, "имя берётся из базового имени URL")
	assert.Equal(t, "https://forum.example.com/downloads/file.bin", att.URL)
}

// TestParsePostWithoutAttachments — ATT-07: пост без вложений → Attachments пустое.
func TestParsePostWithoutAttachments(t *testing.T) {
	page, err := parser.NewXenForo().ParsePage([]byte(xenForoPageHTML(samplePosts())), xenForoTestURL)
	require.NoError(t, err)
	require.Len(t, page.Messages, 2)
	assert.Nil(t, page.Messages[0].Attachments)
	assert.Nil(t, page.Messages[1].Attachments)
}

// TestParseMultipleAttachments — ATT-08: несколько вложений у одного поста сохраняются все.
func TestParseMultipleAttachments(t *testing.T) {
	post := postSpec{
		id:     "400",
		author: "dave",
		date:   "2023-01-01T15:00:00Z",
		body:   "Два файла.",
		number: 4,
		attachments: []attachmentSpec{
			{name: "a.txt", url: "/a.txt", mime: "text/plain", size: "100"},
			{name: "b.png", url: "/b.png", mime: "image/png", size: "200"},
		},
	}
	page, err := parser.NewPHPBB().ParsePage([]byte(phpbbPageHTML([]postSpec{post})), phpbbTestURL)
	require.NoError(t, err)
	require.Len(t, page.Messages, 1)
	require.Len(t, page.Messages[0].Attachments, 2, "оба вложения должны быть извлечены")
	assert.Equal(t, "a.txt", page.Messages[0].Attachments[0].Name)
	assert.Equal(t, "b.png", page.Messages[0].Attachments[1].Name)
}
