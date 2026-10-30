// Package downloader реализует скачивание вложений сообщений согласно
// конфигурационной стратегии (download, links, skip). Файлы записываются
// в поддиректорию <RootDir>/attachments/<postID>/. Тело ответа копируется
// потоком кусками и одновременно вычисляется SHA-256, чтобы не удерживать
// весь ответ в памяти (ограничение: < 50 МБ при файле 100 МБ).
//
// Вложения обрабатываются idempotently: уже существующий файл не
// скачивается повторно (вычисляется только его SHA-256 без обращения к сети).
package downloader

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/kirillshkro/gdumper/internal/model"
	"github.com/kirillshkro/gdumper/internal/storage"
)

// Strategy — стратегия обработки вложений.
type Strategy string

const (
	// StrategyDownload — скачивает файлы в attachments/<postID>/.
	StrategyDownload Strategy = "download"
	// StrategyLinks — не скачивает, сохраняет только URL (LocalPath пуст).
	StrategyLinks Strategy = "links"
	// StrategySkip — не скачивает и не сохраняет вложения.
	StrategySkip Strategy = "skip"
)

var (
	// ErrInvalidStrategy — неизвестная стратегия вложений.
	ErrInvalidStrategy = errors.New("downloader: неизвестная стратегия вложений")
)

// Config — конфигурация Downloader.
type Config struct {
	// Strategy — стратегия обработки вложений.
	Strategy Strategy
	// RootDir — базовая директория; файлы → <RootDir>/attachments/<postID>/
	RootDir string
	// Client — HTTP-клиент; nil → создаётся клиент с таймаутом 30 с и
	// Jar из конфигурации.
	Client *http.Client
	// Jar — куки для запросов вложений (из fetcher/auth). При передаче
	// кастомного Client куки должны быть заданы на нём.
	Jar http.CookieJar
	// UserAgent — агент пользователя для запросов вложений; пустой →
	// дефолтный Go-агент.
	UserAgent string
	// Delay — минимальный интервал между запросами (rate limit); 0 → без лимита.
	Delay time.Duration
	// ChunkSize — размер буфера потокового копирования; 0 → 256 КиБ.
	ChunkSize int64
	// Retries — число повторов при транзитных ошибках; < 0 → 0.
	Retries int
	// RetryBase — база экспоненциального backoff; 0 → 250 мс.
	RetryBase time.Duration
	// Logger — структурированный логгер; nil → slog.Default().
	Logger *slog.Logger
}

// Downloader — скачивает вложения сообщений.
type Downloader struct {
	strategy    Strategy
	rootDir     string
	client      *http.Client
	delay       time.Duration
	chunkSize   int64
	retries     int
	retryBase   time.Duration
	logger      *slog.Logger
	userAgent   string
	mu          sync.Mutex
	lastRequest time.Time
}

// ParseStrategy переводит строковое значение конфигурации в Strategy.
// Неизвестные значения возвращают ErrInvalidStrategy.
func ParseStrategy(mode string) (Strategy, error) {
	switch mode {
	case "download":
		return StrategyDownload, nil
	case "links":
		return StrategyLinks, nil
	case "skip":
		return StrategySkip, nil
	default:
		return "", ErrInvalidStrategy
	}
}

// New создаёт Downloader на основе Config.
func New(cfg Config) *Downloader {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	chunk := cfg.ChunkSize
	if chunk <= 0 {
		chunk = 256 * 1024
	}
	base := cfg.RetryBase
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	retries := cfg.Retries
	if retries < 0 {
		retries = 0
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second, Jar: cfg.Jar}
	}
	return &Downloader{
		strategy:  cfg.Strategy,
		rootDir:   cfg.RootDir,
		client:    client,
		delay:     cfg.Delay,
		chunkSize: chunk,
		retries:   retries,
		retryBase: base,
		logger:    logger,
		userAgent: cfg.UserAgent,
	}
}

// AttachmentsDir возвращает директорию <rootDir>/attachments/<postID>/
// для вложений конкретного поста (имя безопасного postID).
func (d *Downloader) AttachmentsDir(postID string) string {
	return filepath.Join(d.rootDir, "attachments", storage.SanitizeFilename(postID))
}

// job — всё, что нужно для скачивания одного вложения.
type job struct {
	dir      string
	fileName string
	url      string
}

// Download обрабатывает вложения согласно стратегии.
//
// StrategyLinks/StrategySkip: вложения возвращаются как есть — файлы не
// пишутся, LocalPath и SHA256 остаются пустыми.
//
// StrategyDownload: каждое вложение копируется потоком в AttachmentsDir
// (att.MessageID) под именем att.Name. При успехе LocalPath и SHA256
// заполняются in place. Ошибки отдельных вложений (HTTP 4xx/5xx после
// повторов) логируются как warning, вложение остаётся без LocalPath/SHA256,
// и пост продолжает сохраняться. Не nil ошибка возвращается только для
// фатальных проблем поста: неизвестная стратегия, отмена контекста,
// превышение дедлайна.
func (d *Downloader) Download(ctx context.Context, atts []*model.Attachment) ([]*model.Attachment, error) {
	switch d.strategy {
	case StrategyLinks, StrategySkip:
		return atts, nil
	case StrategyDownload:
		break
	default:
		return nil, ErrInvalidStrategy
	}

	for i := range atts {
		att := atts[i]
		if att.MessageID == "" {
			d.logger.Warn("attachment: missing post id, skipped", "url", att.URL)
			continue
		}
		dir := d.AttachmentsDir(att.MessageID)
		fileName := storage.SanitizeFilename(att.Name)
		localPath, sha, err := d.runJob(ctx, &job{dir: dir, fileName: fileName, url: att.URL})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return atts, err
			}
			// Не фатально: warning уже залогирован, идём дальше.
			continue
		}
		atts[i].LocalPath = localPath
		atts[i].SHA256 = sha
	}
	return atts, nil
}
