package config_test

import (
	"regexp"
	"strings"

	"github.com/kirillshkro/gdumper/internal/config"
)

// TestNewConfigDefaults проверяет дефолтные значения NewConfig
func (s *ConfigTestSuite) TestNewConfigDefaults() {
	cfg := config.NewConfig()
	s.Equal("json", cfg.Format)
	s.Equal("all", cfg.Pages)
	s.Equal(1, cfg.Concurrency)
	s.Equal(500, cfg.DelayMS)
	s.Equal("download", cfg.Attachments)
	s.Equal(3, cfg.Retries)
	s.True(strings.HasPrefix(cfg.OutputDir, "./dump_"))
}

// TestNewConfigDefaultOutputDir проверяет формат каталога вывода по умолчанию
func (s *ConfigTestSuite) TestNewConfigDefaultOutputDir() {
	pattern := regexp.MustCompile(`^\./dump_[0-9]{8}_[0-9]{6}$`)
	s.True(pattern.MatchString(config.DefaultOutputDir()),
		"каталог по умолчанию должен иметь вид ./dump_<дата>_<время>")
}

// TestNewConfigValidateWithoutURL проверяет, что конфиг без URL не проходит валидацию
func (s *ConfigTestSuite) TestNewConfigValidateWithoutURL() {
	s.ErrorIs(config.NewConfig().Validate(), config.ErrNoURL)
}

// TestNewOptionsDefaults проверяет дефолтные значения NewOptions
func (s *ConfigTestSuite) TestNewOptionsDefaults() {
	options := config.NewOptions()
	s.Equal("json", options.Format)
	s.False(options.Force)
	s.Empty(options.OutputDir)
}

// TestOptionsValidate проверяет валидацию опций хранилища
func (s *ConfigTestSuite) TestOptionsValidate() {
	empty := config.Options{}
	s.ErrorIs(empty.Validate(), config.ErrNoOutputDir)

	options := config.Options{OutputDir: "/tmp/out"}
	s.ErrorIs(options.Validate(), config.ErrInvalidFormat)

	options.Format = "jsonl"
	s.NoError(options.Validate())
}

// TestConfigCookieSourceAccessor проверяет Config.CookieSource для --cookies
func (s *ConfigTestSuite) TestConfigCookieSourceAccessor() {
	cfg := config.NewConfig()
	src, err := cfg.CookieSource()
	s.NoError(err)
	s.Nil(src)

	cfg.Cookies = "browser:firefox"
	src, err = cfg.CookieSource()
	s.Require().NoError(err)
	s.Equal(config.CookieKindBrowser, src.Kind)
	s.Equal("firefox", src.Browser)
}