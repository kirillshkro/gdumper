package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	// pagesAllRe - диапазон "all"
	pagesAllRe = regexp.MustCompile(`^all$`)
	// pagesNumRe - одиночная страница "3"
	pagesNumRe = regexp.MustCompile(`^[1-9][0-9]*$`)
	// pagesRangeRe - диапазон "1-5" либо открытый "3-"
	pagesRangeRe = regexp.MustCompile(`^[1-9][0-9]*-([1-9][0-9]*)?$`)
)

// Validate валидирует параметры конфигурации перед запуском
func (c *Config) Validate() error {
	validators := []func() error{
		c.validateURL,
		c.validateOutputDir,
		c.validateFormat,
		c.validatePages,
		c.validateCookies,
		c.validateConcurrency,
		c.validateDelay,
		c.validateAttachments,
		c.validateRetries,
	}
	for _, validate := range validators {
		if err := validate(); err != nil {
			return err
		}
	}
	return nil
}

// validateURL валидирует, что URL темы - абсолютная http(s)-ссылка
func (c *Config) validateURL() error {
	if c.URL == "" {
		return fmt.Errorf("%s: %w", FlagURL, ErrNoURL)
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%s %q: %w", FlagURL, c.URL, ErrInvalidURL)
	}
	return nil
}

// validateOutputDir валидирует наличие каталога вывода
func (c *Config) validateOutputDir() error {
	if c.OutputDir == "" {
		return fmt.Errorf("%s: %w", FlagOutputDir, ErrNoOutputDir)
	}
	return nil
}

// validateFormat валидирует формат вывода
func (c *Config) validateFormat() error {
	switch c.Format {
	case "json", "jsonl", "markdown", "html":
		return nil
	}
	return fmt.Errorf("%s %q: %w", FlagFormat, c.Format, ErrInvalidFormat)
}

// validatePages валидирует диапазон страниц (all, N, N-M, N-)
func (c *Config) validatePages() error {
	p := c.Pages
	if pagesAllRe.MatchString(p) || pagesNumRe.MatchString(p) {
		return nil
	}
	if !pagesRangeRe.MatchString(p) {
		return errInvalidPages(p)
	}
	idx := strings.IndexByte(p, '-')
	start, _ := strconv.Atoi(p[:idx])
	rest := p[idx+1:]
	if rest == "" {
		return nil // "N-" - со страницы N до последней
	}
	end, _ := strconv.Atoi(rest)
	if start <= end {
		return nil
	}
	return errInvalidPages(p)
}

// errInvalidPages строит ошибку невалидного диапазона страниц
func errInvalidPages(p string) error {
	return fmt.Errorf("%s %q: %w", FlagPages, p, ErrInvalidPages)
}

// validateCookies валидирует формат источника cookies
func (c *Config) validateCookies() error {
	if _, err := ParseCookies(c.Cookies); err != nil {
		return fmt.Errorf("%s %q: %w", FlagCookies, c.Cookies, err)
	}
	return nil
}

// validateConcurrency валидирует число параллельных запросов
func (c *Config) validateConcurrency() error {
	if c.Concurrency < 1 {
		return fmt.Errorf("%s %d: %w", FlagConcurrency, c.Concurrency, ErrInvalidConcurrency)
	}
	return nil
}

// validateDelay валидирует задержку между запросами
func (c *Config) validateDelay() error {
	if c.DelayMS < 0 {
		return fmt.Errorf("%s %d: %w", FlagDelay, c.DelayMS, ErrInvalidDelay)
	}
	return nil
}

// validateAttachments валидирует режим обработки вложений
func (c *Config) validateAttachments() error {
	switch c.Attachments {
	case "download", "links", "skip":
		return nil
	}
	return fmt.Errorf("%s %q: %w", FlagAttachments, c.Attachments, ErrInvalidAttachments)
}

// validateRetries валидирует число повторов
func (c *Config) validateRetries() error {
	if c.Retries < 0 {
		return fmt.Errorf("%s %d: %w", FlagRetries, c.Retries, ErrInvalidRetries)
	}
	return nil
}
