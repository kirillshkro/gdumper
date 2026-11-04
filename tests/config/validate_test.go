package config_test

import (
	"github.com/kirillshkro/gdumper/internal/config"
)

// newValidConfig строит корректную конфигурацию для тестов валидации
func (s *ConfigTestSuite) newValidConfig() *config.Config {
	cfg := config.NewConfig()
	cfg.URL = testURL
	return cfg
}

// TestValidateURL проверяет валидацию URL
func (s *ConfigTestSuite) TestValidateURL() {
	valid := s.newValidConfig()
	s.NoError(valid.Validate())

	valid.URL = ""
	s.ErrorIs(valid.Validate(), config.ErrNoURL)

	valid.URL = "invalid-url"
	s.ErrorIs(valid.Validate(), config.ErrInvalidURL)

	valid.URL = "ftp://forum.example/threads/123/"
	s.ErrorIs(valid.Validate(), config.ErrInvalidURL)
}

// TestValidateOutputDir проверяет валидацию каталога вывода
func (s *ConfigTestSuite) TestValidateOutputDir() {
	valid := s.newValidConfig()
	s.NoError(valid.Validate())

	valid.OutputDir = ""
	s.ErrorIs(valid.Validate(), config.ErrNoOutputDir)
}

// TestValidateFormat проверяет валидацию формата вывода
func (s *ConfigTestSuite) TestValidateFormat() {
	valid := s.newValidConfig()
	for _, format := range []string{"json", "jsonl", "markdown", "html"} {
		valid.Format = format
		s.NoError(valid.Validate())
	}
	valid.Format = "xml"
	s.ErrorIs(valid.Validate(), config.ErrInvalidFormat)
}

// TestValidatePages проверяет валидацию диапазона страниц
func (s *ConfigTestSuite) TestValidatePages() {
	valid := s.newValidConfig()
	for _, pages := range []string{"all", "1", "3", "3-", "1-5"} {
		valid.Pages = pages
		s.NoError(valid.Validate())
	}
	for _, pages := range []string{"5-2", "0", "abc", "", "1-5-3"} {
		valid.Pages = pages
		s.ErrorIs(valid.Validate(), config.ErrInvalidPages)
	}
}

// TestValidateCookies проверяет валидацию источника cookies
func (s *ConfigTestSuite) TestValidateCookies() {
	valid := s.newValidConfig()
	s.NoError(valid.Validate())

	valid.Cookies = "browser:ie"
	s.ErrorIs(valid.Validate(), config.ErrUnknownBrowser)

	valid.Cookies = "weird:source"
	s.ErrorIs(valid.Validate(), config.ErrUnknownCookieKind)
}

// TestValidateNumbers проверяет валидацию числовых параметров
func (s *ConfigTestSuite) TestValidateNumbers() {
	valid := s.newValidConfig()
	s.NoError(valid.Validate())

	valid.Concurrency = 0
	s.ErrorIs(valid.Validate(), config.ErrInvalidConcurrency)

	valid = s.newValidConfig()
	valid.DelayMS = -1
	s.ErrorIs(valid.Validate(), config.ErrInvalidDelay)

	valid = s.newValidConfig()
	valid.Retries = -1
	s.ErrorIs(valid.Validate(), config.ErrInvalidRetries)
}

// TestValidateAttachments проверяет валидацию режима обработки вложений
func (s *ConfigTestSuite) TestValidateAttachments() {
	valid := s.newValidConfig()
	for _, attachments := range []string{"download", "links", "skip"} {
		valid.Attachments = attachments
		s.NoError(valid.Validate())
	}
	valid.Attachments = "other"
	s.ErrorIs(valid.Validate(), config.ErrInvalidAttachments)
}

// TestValidateProxy проверяет валидацию адреса SOCKS5-прокси: пустое значение
// и socks5/socks5h с ненулевым хостом допустимы, остальные варианты — ошибка
// ErrInvalidProxy.
func (s *ConfigTestSuite) TestValidateProxy() {
	// пустая и допустимые адреса.
	s.NoError(s.newValidConfig().Validate())
	for _, proxy := range []string{
		"socks5://proxy.example.com:1080",
		"socks5://127.0.0.1:1080",
		"socks5://host",
		"socks5://user:pass@host:1080",
		"socks5h://proxy.example.com:1080",
		"socks5h://host",
	} {
		c := s.newValidConfig()
		c.Proxy = proxy
		s.NoError(c.Validate(), "valid proxy %q", proxy)
	}

	// socks5/socks5h без хоста.
	for _, proxy := range []string{
		"socks5://",
		"socks5://:1080",
		"socks5h://",
		"socks5h://:1080",
	} {
		c := s.newValidConfig()
		c.Proxy = proxy
		s.ErrorIs(c.Validate(), config.ErrInvalidProxy, "missing host %q", proxy)
	}

	// не поддержанные схемы.
	for _, proxy := range []string{
		"http://proxy.example.com:3128",
		"https://proxy.example.com:3128",
		"socks4://proxy.example.com:1080",
		"ftp://proxy.example.com:21",
		"not-a-url",
	} {
		c := s.newValidConfig()
		c.Proxy = proxy
		s.ErrorIs(c.Validate(), config.ErrInvalidProxy, "unsupported scheme %q", proxy)
	}

	// невалидная кодировка URL.
	c := s.newValidConfig()
	c.Proxy = "socks5://host%zz"
	s.ErrorIs(c.Validate(), config.ErrInvalidProxy, "invalid percent-encoding")
}