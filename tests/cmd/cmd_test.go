package cmd

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// cliPackage - путь к пакету CLI
const cliPackage = "github.com/kirillshkro/gdumper/cmd"

// testURL - корректный URL темы
const testURL = "https://example.com/threads/1/"

// cliResult - результат выполнения CLI
type cliResult struct {
	output   string
	exitCode int
	err      error
}

// CmdTestSuite представляет тестовый комплект для CLI команды
type CmdTestSuite struct {
	suite.Suite
	binPath string
}

// TestCmdSuite запускает тесты
func TestCmdSuite(t *testing.T) {
	suite.Run(t, new(CmdTestSuite))
}

// SetupSuite собирает бинарник CLI один раз перед всеми тестами.
// go run не проксирует exit code дочернего процесса (при ненулевом
// коде программы сам завершается кодом 1), поэтому тесты запускают
// собранный бинарник напрямую.
func (s *CmdTestSuite) SetupSuite() {
	s.binPath = filepath.Join(s.T().TempDir(), "forumdump")
	build := exec.Command("go", "build", "-o", s.binPath, cliPackage)
	output, err := build.CombinedOutput()
	s.Require().NoError(err, "не удалось собрать CLI: %s", output)
}

// runCLI выполняет собранный CLI и возвращает её вывод, exit code и ошибку
func (s *CmdTestSuite) runCLI(args ...string) cliResult {
	cmd := exec.Command(s.binPath, args...)
	output, err := cmd.CombinedOutput()
	result := cliResult{output: string(output), err: err}
	switch typedErr := err.(type) {
	case *exec.ExitError:
		result.exitCode = typedErr.ExitCode()
	case nil:
		// процесс завершился с кодом 0
	default:
		result.exitCode = -1
	}
	return result
}

// TestHelpFlag тестирует вывод --help (TC-01.2)
func (s *CmdTestSuite) TestHelpFlag() {
	res := s.runCLI("--help")
	s.NoError(res.err, "Команда --help должна завершиться успешно")
	s.Contains(res.output, "Usage:")
	s.Contains(res.output, "Flags:")

	expectedFlags := []string{
		"--url", "--cookies", "--out", "--format", "--pages", "--concurrency",
		"--delay", "--attachments", "--retries", "--user-agent", "--config", "--verbose",
	}
	for _, flag := range expectedFlags {
		s.Contains(res.output, flag, "Вывод --help должен содержать флаг: "+flag)
	}
}

// TestMissingURL тестирует запуск без --url (TC-01.3)
func (s *CmdTestSuite) TestMissingURL() {
	res := s.runCLI()
	s.Equal(2, res.exitCode, "Запуск без --url должен завершиться кодом 2")
	s.Contains(res.output, "error")
}

// TestInvalidURL тестирует невалидный URL (TC-01.4)
func (s *CmdTestSuite) TestInvalidURL() {
	res := s.runCLI("--url", "invalid-url")
	s.Equal(2, res.exitCode, "Невалидный URL должен завершиться кодом 2")
	s.Contains(res.output, "invalid")
}

// TestUnknownFormat тестирует неизвестный формат (TC-01.5)
func (s *CmdTestSuite) TestUnknownFormat() {
	res := s.runCLI("--url", testURL, "--format", "unknown")
	s.Equal(2, res.exitCode, "Неизвестный формат должен завершиться кодом 2")
	s.Contains(res.output, "format")
}

// TestInvalidPagesRange тестирует невалидный диапазон страниц (TC-01.6)
func (s *CmdTestSuite) TestInvalidPagesRange() {
	res := s.runCLI("--url", testURL, "--pages", "5-2")
	s.Equal(2, res.exitCode, "Невалидный диапазон страниц должен завершиться кодом 2")
	s.Contains(res.output, "pages")
}

// TestEnvironmentVariableOverride тестирует подстановку FORUMDUMP_URL из ENV (TC-01.6)
func (s *CmdTestSuite) TestEnvironmentVariableOverride() {
	// Без ENV-переменной - URL отсутствует, exit code 2
	res := s.runCLI()
	s.Equal(2, res.exitCode, "Без FORUMDUMP_URL команда должна завершиться кодом 2")

	// С ENV-переменной - URL берется из FORUMDUMP_URL, команда успешна
	s.T().Setenv("FORUMDUMP_URL", "https://example.com/test")
	res = s.runCLI()
	s.NoError(res.err, "С FORUMDUMP_URL команда должна завершиться успешно")
}
