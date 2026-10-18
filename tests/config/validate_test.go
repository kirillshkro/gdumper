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