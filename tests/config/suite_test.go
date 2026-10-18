package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/suite"

	"github.com/kirillshkro/gdumper/internal/config"
)

// ConfigTestSuite - тестовый комплект пакета internal/config
type ConfigTestSuite struct {
	suite.Suite
}

// TestConfigSuite запускает тестовый комплект
func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

// newFlags создает набор флагов со всеми флагами из ТЗ
func (s *ConfigTestSuite) newFlags() *pflag.FlagSet {
	return config.NewFlagSet()
}

// setFlags устанавливает значения флагов
func (s *ConfigTestSuite) setFlags(fs *pflag.FlagSet, values map[string]string) {
	for name, value := range values {
		s.Require().NoError(fs.Set(name, value))
	}
}

// writeConfigFile пишет YAML/JSON-файл конфигурации и возвращает путь
func (s *ConfigTestSuite) writeConfigFile(name, content string) string {
	path := filepath.Join(s.T().TempDir(), name)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
	return path
}
