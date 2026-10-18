package config_test

import (
	"strings"

	"github.com/kirillshkro/gdumper/internal/config"
)

// testURL - корректный URL темы для тестов
const testURL = "https://forum.example/threads/123/"

// loadConfig собирает флаги, устанавливает значения и загружает конфигурацию
func (s *ConfigTestSuite) loadConfig(values map[string]string) (*config.Config, error) {
	fs := s.newFlags()
	s.setFlags(fs, values)
	return config.Load(fs)
}

// TestLoadDefaults проверяет значения по умолчанию при заданном только URL
func (s *ConfigTestSuite) TestLoadDefaults() {
	cfg, err := s.loadConfig(map[string]string{config.FlagURL: testURL})
	s.Require().NoError(err)
	s.Equal(testURL, cfg.URL)
	s.Equal("json", cfg.Format)
	s.Equal("all", cfg.Pages)
	s.Equal(1, cfg.Concurrency)
	s.Equal(500, cfg.DelayMS)
	s.Equal("download", cfg.Attachments)
	s.Equal(3, cfg.Retries)
	s.False(cfg.Verbose)
	s.Empty(cfg.UserAgent)
	s.Empty(cfg.Cookies)
	s.Empty(cfg.ConfigFile)
	s.True(strings.HasPrefix(cfg.OutputDir, "./dump_"))
}

// TestLoadFlagValues проверяет загрузку значений всех флагов
func (s *ConfigTestSuite) TestLoadFlagValues() {
	cfg, err := s.loadConfig(map[string]string{
		config.FlagURL:         testURL,
		config.FlagCookies:     "browser:chrome",
		config.FlagOutputDir:   "/tmp/dump",
		config.FlagFormat:      "jsonl",
		config.FlagPages:       "3-",
		config.FlagConcurrency: "2",
		config.FlagDelay:       "100",
		config.FlagAttachments: "skip",
		config.FlagRetries:     "5",
		config.FlagUserAgent:   "forumdump/1.0",
		config.FlagVerbose:     "true",
	})
	s.Require().NoError(err)
	s.Equal("/tmp/dump", cfg.OutputDir)
	s.Equal("jsonl", cfg.Format)
	s.Equal("3-", cfg.Pages)
	s.Equal(2, cfg.Concurrency)
	s.Equal(100, cfg.DelayMS)
	s.Equal("skip", cfg.Attachments)
	s.Equal(5, cfg.Retries)
	s.Equal("forumdump/1.0", cfg.UserAgent)
	s.True(cfg.Verbose)
}

// TestLoadEnvOverride проверяет подстановку значений из переменных окружения (FORUMDUMP_*)
func (s *ConfigTestSuite) TestLoadEnvOverride() {
	s.T().Setenv("FORUMDUMP_FORMAT", "markdown")
	s.T().Setenv("FORUMDUMP_USER_AGENT", "env-ua")

	cfg, err := s.loadConfig(map[string]string{config.FlagURL: testURL})
	s.Require().NoError(err)
	s.Equal("markdown", cfg.Format)
	s.Equal("env-ua", cfg.UserAgent)
}

// TestLoadFlagOverridesEnv проверяет приоритет флагов над переменными окружения
func (s *ConfigTestSuite) TestLoadFlagOverridesEnv() {
	s.T().Setenv("FORUMDUMP_FORMAT", "markdown")

	cfg, err := s.loadConfig(map[string]string{
		config.FlagURL:    testURL,
		config.FlagFormat: "jsonl",
	})
	s.Require().NoError(err)
	s.Equal("jsonl", cfg.Format)
}

// TestLoadConfigFileYAML проверяет загрузку YAML-файла конфигурации
func (s *ConfigTestSuite) TestLoadConfigFileYAML() {
	path := s.writeConfigFile("config.yaml",
		"format: jsonl\npages: \"1-5\"\ndelay: 100\nconcurrency: 2\n")

	cfg, err := s.loadConfig(map[string]string{
		config.FlagURL:    testURL,
		config.FlagConfig: path,
	})
	s.Require().NoError(err)
	s.Equal("jsonl", cfg.Format)
	s.Equal("1-5", cfg.Pages)
	s.Equal(100, cfg.DelayMS)
	s.Equal(2, cfg.Concurrency)
	s.Equal(path, cfg.ConfigFile)
}

// TestLoadConfigFileJSON проверяет загрузку JSON-файла конфигурации
func (s *ConfigTestSuite) TestLoadConfigFileJSON() {
	path := s.writeConfigFile("config.json", `{"format":"markdown","retries":7}`)

	cfg, err := s.loadConfig(map[string]string{
		config.FlagURL:    testURL,
		config.FlagConfig: path,
	})
	s.Require().NoError(err)
	s.Equal("markdown", cfg.Format)
	s.Equal(7, cfg.Retries)
}

// TestLoadCascade проверяет каскад приоритетов: флаги > окружение > файл
func (s *ConfigTestSuite) TestLoadCascade() {
	s.T().Setenv("FORUMDUMP_FORMAT", "markdown")
	path := s.writeConfigFile("cascade.yaml", "format: jsonl\n")

	cfg, err := s.loadConfig(map[string]string{
		config.FlagURL:    testURL,
		config.FlagFormat: "html",
		config.FlagConfig: path,
	})
	s.Require().NoError(err)
	s.Equal("html", cfg.Format)
}

// TestLoadInvalidConfigFile проверяет ошибку при отсутствующем файле конфигурации
func (s *ConfigTestSuite) TestLoadInvalidConfigFile() {
	_, err := s.loadConfig(map[string]string{
		config.FlagURL:    testURL,
		config.FlagConfig: "/nonexistent/config.yaml",
	})
	s.Error(err)
}

// TestLoadValidationErrors проверяет, что ошибки валидации пробрасываются из Load
func (s *ConfigTestSuite) TestLoadValidationErrors() {
	_, err := s.loadConfig(map[string]string{})
	s.ErrorIs(err, config.ErrNoURL)

	_, err = s.loadConfig(map[string]string{config.FlagURL: "invalid-url"})
	s.ErrorIs(err, config.ErrInvalidURL)

	_, err = s.loadConfig(map[string]string{
		config.FlagURL:   testURL,
		config.FlagPages: "5-2",
	})
	s.ErrorIs(err, config.ErrInvalidPages)
}