// Package main — основной сценарий обхода темы (`forumdump`).
//
// runPipeline связывает все модули: авторизация (auth), HTTP-клиент
// (fetcher), определение движка и пагинация (parser/paginator), скачивание
// вложений (downloader) и сохранение артефактов (storage). Возвращает код
// завершения по ТЗ (раздел 6).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/kirillshkro/gdumper/internal/auth"
	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/downloader"
	"github.com/kirillshkro/gdumper/internal/fetcher"
	"github.com/kirillshkro/gdumper/internal/model"
	"github.com/kirillshkro/gdumper/internal/paginator"
	"github.com/kirillshkro/gdumper/internal/parser"
	"github.com/kirillshkro/gdumper/internal/storage"
)

// toolVersion — версия инструмента, фиксируемая в manifest.json.
const toolVersion = "1.0.0"

// manifest — метаданные обхода, сохраняемые в out/manifest.json (ТЗ раздел 5).
type manifest struct {
	DumpedAt     string   `json:"dumped_at"`
	PagesFetched int      `json:"pages_fetched"`
	Errors       []string `json:"errors"`
	Incomplete   bool     `json:"incomplete"`
	ToolVersion  string   `json:"tool_version"`
}

// runPipeline выполняет полный сценарий обхода темы и возвращает код завершения.
//
// Порядок: авторизация → HTTP-клиент → определение движка и пагинация →
// запись данных → manifest → скачивание вложений.
func runPipeline(cfg *config.Config, logger *slog.Logger) int {
	// Общий контекст: Ctrl+C gracefully останавливает обход (ТЗ раздел 7).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 1. Авторизация: загрузка cookies (если запрошено). Ошибка → exit 4.
	jar, err := loadCookies(cfg, logger)
	if err != nil {
		logger.Error("authentication failed", "error", err)
		return exitAuth
	}

	// 2. HTTP-клиент и обход (определение движка, пагинация).
	client := newFetcher(cfg, jar, logger)
	summary, results, crawlErr := crawl(cfg, client, logger, ctx)

	// 3. Сборка контента. saveData — первая запись в каталог: storage
	//    проверяет, что каталог не занят чужими данными (ErrDirectoryExists),
	//    поэтому данные записываются до manifest'а и вложений.
	topic, messages := aggregate(results)
	if err := saveData(cfg, logger, topic, messages); err != nil {
		logger.Error("failed to write output", "error", err)
		return exitDisk
	}

	// 4. Запись manifest.json (каталог уже создан и заполнен данными).
	if err := writeManifest(cfg, logger, summary, ctx.Err() != nil); err != nil {
		logger.Error("failed to write manifest", "error", err)
		return exitDisk
	}

	// 5. Скачивание вложений (создаёт attachments/<postID>/ через os.MkdirAll,
	//    не блокируясь проверкой non-empty каталога storage).
	atts := flattenAttachments(messages)
	if len(atts) > 0 {
		dl := newDownloader(cfg, jar, logger)
		_, _ = dl.Download(ctx, atts)
	}

	if crawlErr != nil {
		logger.Warn("crawl finished with issues", "error", crawlErr)
	} else {
		logger.Info("dump complete",
			"planned", summary.Planned,
			"fetched", summary.Fetched,
			"failed", summary.Failed,
			"messages", len(messages))
	}
	return exitCodeForCrawl(summary)
}

// exitCodeForCrawl — коды завершения по ТЗ (раздел 6).
//
// exit 0 — процесс завершился и артефакты записаны (в т.ч. когда ничего
//
//	не удалось извлечь: причина видна в manifest, incomplete=true).
//
// exit 5 — извлечена часть контента, но обход не был полным (предупреждения).
func exitCodeForCrawl(s paginator.Summary) int {
	if s.Fetched == 0 {
		return exitSuccess
	}
	if s.Failed > 0 || len(s.Errors) > 0 || s.Fetched < s.Planned {
		return exitPartial
	}
	return exitSuccess
}

// crawl — загружает первую страницу, определяет движок и обходит тему.
//
// Возвращает сводку (Summary), данные страниц и фатальную ошибку, если
// обход не мог даже начаться (недоступная первая страница, неизвестный движок).
func crawl(cfg *config.Config, client *fetcher.Client, logger *slog.Logger, ctx context.Context) (paginator.Summary, []paginator.PageData, error) {
	// Первая страница задаёт канонический URL темы и позволяет выбрать движок.
	first, err := client.Fetch(ctx, cfg.URL)
	if err != nil {
		msg := fmt.Errorf("first page not reachable: %w", err)
		return paginator.Summary{Errors: []error{msg}}, nil, msg
	}
	if first.IsHTTPError() {
		msg := fmt.Errorf("first page unavailable (HTTP %d)", first.StatusCode)
		return paginator.Summary{Errors: []error{msg}}, nil, msg
	}

	base := first.FinalURL
	p, ok := detectParser(first.Body, base)
	if !ok {
		logger.Warn("no known forum parser matched the first page; nothing will be crawled")
		return paginator.Summary{Errors: []error{parser.ErrNoParser}}, nil, parser.ErrNoParser
	}

	pg := paginator.New(client, p, base)
	if err := pg.SetRange(cfg.Pages); err != nil {
		return paginator.Summary{Errors: []error{err}}, nil, err
	}
	summary, results := pg.Walk(ctx)
	logger.Info("pagination walk", "planned", summary.Planned, "fetched", summary.Fetched, "failed", summary.Failed)
	return summary, results, nil
}

// detectParser — выбирает первый из поддерживаемых парсеров, распознающий страницу.
func detectParser(html []byte, url string) (parser.Parser, bool) {
	for _, p := range parser.Parsers {
		if p.Match(html, url) {
			return p, true
		}
	}
	return nil, false
}

// aggregate — объединяет результаты страниц в тему и плоский список сообщений.
func aggregate(results []paginator.PageData) (*model.Topic, []*model.Message) {
	var topic *model.Topic
	var messages []*model.Message
	for _, r := range results {
		if r.Data == nil {
			continue
		}
		if topic == nil && r.Data.Topic != nil {
			topic = r.Data.Topic
		}
		messages = append(messages, r.Data.Messages...)
	}
	if topic == nil {
		topic = &model.Topic{}
	}
	return topic, messages
}

// flattenAttachments — извлекает все вложения из сообщений в плоский список
// (управление ими — за downloader'ом).
func flattenAttachments(messages []*model.Message) []*model.Attachment {
	var out []*model.Attachment
	for _, m := range messages {
		out = append(out, m.Attachments...)
	}
	return out
}

// loadCookies — загружает cookie-jar из источника, заданного в --cookies.
//
// Без источника cookies возвращает (nil, nil) — запрос выполняется без авторизации.
func loadCookies(cfg *config.Config, logger *slog.Logger) (http.CookieJar, error) {
	source, err := cfg.CookieSource()
	if err != nil {
		return nil, fmt.Errorf("cookies: parse %q: %w", cfg.Cookies, err)
	}
	if source == nil {
		return nil, nil
	}
	return auth.Load(source, logger)
}

// newFetcher — HTTP-клиент fetcher с cookies, задержкой и retry из конфига.
func newFetcher(cfg *config.Config, jar http.CookieJar, logger *slog.Logger) *fetcher.Client {
	return fetcher.New(config.FetchOptions{
		Timeout:   30 * time.Second,
		Logger:    logger,
		Delay:     time.Duration(cfg.DelayMS) * time.Millisecond,
		Retry:     cfg.Retries,
		UserAgent: cfg.UserAgent,
		Jar:       jar,
	})
}

// newDownloader — Downloader со стратегией обработки вложений из --attachments.
func newDownloader(cfg *config.Config, jar http.CookieJar, logger *slog.Logger) *downloader.Downloader {
	strategy := downloader.StrategySkip
	if s, err := downloader.ParseStrategy(cfg.Attachments); err == nil {
		strategy = s
	}
	return downloader.New(downloader.Config{
		Strategy:  strategy,
		RootDir:   cfg.OutputDir,
		Jar:       jar,
		UserAgent: cfg.UserAgent,
		Delay:     time.Duration(cfg.DelayMS) * time.Millisecond,
		Retries:   cfg.Retries,
		Logger:    logger,
	})
}

// dataFileName — имя файла с данными в зависимости от --format.
func dataFileName(format string) string {
	switch format {
	case "json":
		return "topic.json"
	case "jsonl":
		return "topic.jsonl"
	case "markdown":
		return "topic.markdown"
	case "html":
		return "topic.html"
	default:
		return "topic.json"
	}
}

// saveData — записывает собранные данные в файл через storage.
//
// Вложения уже объединены с сообщениями (в них вписаны LocalPath/SHA256
// после работы downloader'а), поэтому третий аргумент Save — nil.
func saveData(cfg *config.Config, logger *slog.Logger, topic *model.Topic, messages []*model.Message) error {
	s := storage.NewStorage(config.Options{OutputDir: cfg.OutputDir, Format: cfg.Format})
	if err := s.Save(topic, messages, nil); err != nil {
		return fmt.Errorf("save data: %w", err)
	}
	return s.WriteToFile(filepath.Join(cfg.OutputDir, dataFileName(cfg.Format)))
}

// writeManifest — фиксирует результат обхода в out/manifest.json (ТЗ раздел 5).
func writeManifest(cfg *config.Config, logger *slog.Logger, s paginator.Summary, interrupted bool) error {
	var errs []string
	for _, e := range s.Errors {
		errs = append(errs, e.Error())
	}
	incomplete := s.Failed > 0 || len(errs) > 0 || s.Fetched < s.Planned || interrupted
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", cfg.OutputDir, err)
	}
	data, err := json.MarshalIndent(manifest{
		DumpedAt:     time.Now().UTC().Format(time.RFC3339),
		PagesFetched: s.Fetched,
		Errors:       errs,
		Incomplete:   incomplete,
		ToolVersion:  toolVersion,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	logger.Info("manifest written", "pages_fetched", s.Fetched, "incomplete", incomplete)
	return nil
}
