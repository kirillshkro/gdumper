package config

import (
	"errors"
	"log/slog"
	"time"
)

var (
	// ErrNoOutputDir - ошибка отсутствия выходного каталога
	ErrNoOutputDir = errors.New("output directory not specified")

	// ErrInvalidFormat - ошибка невалидного формата
	ErrInvalidFormat = errors.New("invalid output format")

	// ErrNoURL - ошибка отсутствия URL темы
	ErrNoURL = errors.New("url is required")

	// ErrInvalidURL - ошибка невалидного URL темы
	ErrInvalidURL = errors.New("invalid url")

	// ErrInvalidPages - ошибка невалидного диапазона страниц
	ErrInvalidPages = errors.New("invalid pages range")

	// ErrInvalidAttachments - ошибка невалидного режима обработки вложений
	ErrInvalidAttachments = errors.New("invalid attachments mode")

	// ErrInvalidConcurrency - ошибка невалидного числа параллельных запросов
	ErrInvalidConcurrency = errors.New("invalid concurrency value")

	// ErrInvalidDelay - ошибка невалидной задержки между запросами
	ErrInvalidDelay = errors.New("invalid delay value")

	// ErrInvalidRetries - ошибка невалидного числа повторов
	ErrInvalidRetries = errors.New("invalid retries value")
)

// Config - единая конфигурация приложения (все флаги ТЗ 2.1)
type Config struct {
	URL         string `mapstructure:"url" json:"url"`
	Cookies     string `mapstructure:"cookies" json:"cookies"`
	OutputDir   string `mapstructure:"out" json:"out"`
	Format      string `mapstructure:"format" json:"format"`
	Pages       string `mapstructure:"pages" json:"pages"`
	Concurrency int    `mapstructure:"concurrency" json:"concurrency"`
	DelayMS     int    `mapstructure:"delay" json:"delay"`
	Attachments string `mapstructure:"attachments" json:"attachments"`
	Retries     int    `mapstructure:"retries" json:"retries"`
	UserAgent   string `mapstructure:"user-agent" json:"user-agent"`
	ConfigFile  string `mapstructure:"config" json:"config"`
	Verbose     bool   `mapstructure:"verbose" json:"verbose"`
}

// Options представляет конфигурационные опции хранилища
type Options struct {
	OutputDir string `mapstructure:"output_dir" json:"output_dir"`
	Format    string `mapstructure:"format" json:"format"`
	Force     bool   `mapstructure:"force" json:"force"`
}

// NewConfig создает конфигурацию с дефолтными значениями (ТЗ 2.1)
func NewConfig() *Config {
	return &Config{
		OutputDir:   DefaultOutputDir(),
		Format:      "json",
		Pages:       "all",
		Concurrency: 1,
		DelayMS:     500,
		Attachments: "download",
		Retries:     3,
	}
}

// DefaultOutputDir строит каталог вывода по умолчанию с таймстемпом
func DefaultOutputDir() string {
	return "./dump_" + time.Now().Format("20060102_150405")
}

// NewOptions создает новые опции с дефолтными значениями
func NewOptions() *Options {
	return &Options{
		Format: "json",
	}
}

// Validate валидирует опции хранилища
func (o *Options) Validate() error {
	if o.OutputDir == "" {
		return ErrNoOutputDir
	}

	validFormats := map[string]bool{
		"json":     true,
		"jsonl":    true,
		"markdown": true,
		"html":     true,
	}

	if !validFormats[o.Format] {
		return ErrInvalidFormat
	}

	return nil
}

// FetchOptions are the fetcher Client initialization parameters.
type FetchOptions struct {
	// Timeout is the per-request timeout; 0 means no timeout.
	Timeout time.Duration
	// Logger is a structured logger; nil means slog.Default().
	Logger *slog.Logger
	// Delay is the minimum interval between consecutive requests
	// (RPS limit); 0 means no limit.
	Delay time.Duration
	// Retry is the number of retry attempts after the initial attempt;
	// negative values are treated as 0.
	Retry int
	// RetryBase is the base of the exponential backoff; 0 means 250ms.
	RetryBase time.Duration
	// UserAgent is the User-Agent header value; "" means the transport default.
	UserAgent string
	// Headers are extra headers set on every request (before Accept and
	// Accept-Language).
	Headers map[string]string
}

// NewFetchOptions returns the FetchOptions with default values.
func NewFetchOptions() FetchOptions {
	return FetchOptions{
		Timeout:   30 * time.Second,
		Logger:    slog.Default(),
		Delay:     500 * time.Millisecond,
		Retry:     3,
		RetryBase: 250 * time.Millisecond,
	}
}
