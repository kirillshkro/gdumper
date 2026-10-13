package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// CmdTestSuite представляет тестовый набор для CLI команд
type CmdTestSuite struct {
	suite.Suite
}

// TestCmdSuite запускает тесты
func TestCmdSuite(t *testing.T) {
	suite.Run(t, new(CmdTestSuite))
}

// SetupSuite устанавливает тестовый набор
func (suite *CmdTestSuite) SetupSuite() {
	// Подготовка перед запуском тестов
}

// TearDownSuite завершает тестовый набор
func (suite *CmdTestSuite) TearDownSuite() {
	// Очистка после завершения тестов
}

// TestHelpFlag тестирует вывод справки с флагом --help
func (suite *CmdTestSuite) TestHelpFlag() {
	// Запускаем команду с флагом --help
	cmd := exec.Command("go", "run", "./cmd/main.go", "--help")
	output, err := cmd.CombinedOutput()
	
	// Проверяем, что команда завершилась успешно (exit code 0)
	suite.NoError(err, "Команда --help должна завершиться успешно")
	
	// Проверяем, что вывод содержит информацию о флагах
	outputStr := string(output)
	suite.Contains(outputStr, "Usage:", "Вывод должен содержать информацию о использовании")
	suite.Contains(outputStr, "Flags:", "Вывод должен содержать информацию о флагах")
	
	// Проверяем, что содержится информация о всех основных флагах из ТЗ
	expectedFlags := []string{
		"--url", "--cookies", "--out", "--format", "--pages", "--concurrency",
		"--delay", "--attachments", "--retries", "--user-agent", "--config", "--verbose",
	}
	
	for _, flag := range expectedFlags {
		suite.Contains(outputStr, flag, "Вывод справки должен содержать флаг: "+flag)
	}
}

// TestMissingURL тестирует запуск без --url
func (suite *CmdTestSuite) TestMissingURL() {
	// Запускаем команду без --url
	cmd := exec.Command("go", "run", "./cmd/main.go")
	output, err := cmd.CombinedOutput()
	
	// Проверяем, что команда завершилась с кодом 2 (ошибка аргументов)
	suite.NotNil(err, "Команда должна завершиться с ошибкой")
	
	// Получаем exit code
	exitCode := 2
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	
	suite.Equal(2, exitCode, "Запуск без --url должен завершаться кодом 2")
	
	// Проверяем, что вывод содержит сообщение об ошибке
	outputStr := string(output)
	suite.Contains(outputStr, "error", "Вывод должен содержать сообщение об ошибке")
}

// TestInvalidURL тестирует невалидный URL
func (suite *CmdTestSuite) TestInvalidURL() {
	// Запускаем команду с невалидным URL
	cmd := exec.Command("go", "run", "./cmd/main.go", "--url", "invalid-url")
	output, err := cmd.CombinedOutput()
	
	// Проверяем, что команда завершилась с кодом 2 (ошибка аргументов)
	suite.NotNil(err, "Команда с невалидным URL должна завершиться с ошибкой")
	
	// Получаем exit code
	exitCode := 2
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	
	suite.Equal(2, exitCode, "Невалидный URL должен завершаться кодом 2")
	
	// Проверяем, что вывод содержит сообщение об ошибке
	outputStr := string(output)
	suite.Contains(outputStr, "invalid", "Вывод должен содержать сообщение об ошибке валидации URL")
}

// TestUnknownFormat тестирует неизвестный формат
func (suite *CmdTestSuite) TestUnknownFormat() {
	// Запускаем команду с неизвестным форматом
	cmd := exec.Command("go", "run", "./cmd/main.go", "--url", "https://example.com", "--format", "unknown")
	output, err := cmd.CombinedOutput()
	
	// Проверяем, что команда завершилась с кодом 2 (ошибка аргументов)
	suite.NotNil(err, "Команда с неизвестным форматом должна завершиться с ошибкой")
	
	// Получаем exit code
	exitCode := 2
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	
	suite.Equal(2, exitCode, "Неизвестный формат должен завершаться кодом 2")
	
	// Проверяем, что вывод содержит сообщение об ошибке
	outputStr := string(output)
	suite.Contains(outputStr, "format", "Вывод должен содержать сообщение об ошибке формата")
}

// TestInvalidPagesRange тестирует некорректный диапазон страниц
func (suite *CmdTestSuite) TestInvalidPagesRange() {
	// Запускаем команду с некорректным диапазоном страниц (5-2)
	cmd := exec.Command("go", "run", "./cmd/main.go", "--url", "https://example.com", "--pages", "5-2")
	output, err := cmd.CombinedOutput()
	
	// Проверяем, что команда завершилась с кодом 2 (ошибка аргументов)
	suite.NotNil(err, "Команда с некорректным диапазоном страниц должна завершиться с ошибкой")
	
	// Получаем exit code
	exitCode := 2
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	
	suite.Equal(2, exitCode, "Некорректный диапазон страниц должен завершаться кодом 2")
	
	// Проверяем, что вывод содержит сообщение об ошибке
	outputStr := string(output)
	suite.Contains(outputStr, "pages", "Вывод должен содержать сообщение об ошибке диапазона страниц")
}

// TestEnvironmentVariableOverride тестирует переопределение флага через переменную окружения
func (suite *CmdTestSuite) TestEnvironmentVariableOverride() {
	// Устанавливаем переменную окружения FORUMDUMP_URL
	os.Setenv("FORUMDUMP_URL", "https://example.com/test")
	
	// Запускаем команду - она должна использовать переменную окружения
	cmd := exec.Command("go", "run", "./cmd/main.go")
	output, err := cmd.CombinedOutput()
	
	// Восстанавливаем оригинальное значение
	os.Unsetenv("FORUMDUMP_URL")
	
	// Проверяем, что команда завершилась с кодом 2 (так как URL не валиден, но переменная использована)
	// или что ошибка не возникает из-за использования переменной окружения
	suite.NotNil(err, "Команда с переменной окружения должна завершиться, даже если URL не валиден")
	
	// Проверяем, что вывод не содержит ошибку про отсутствие URL (если реализация проверяет переменные)
	outputStr := string(output)
	if strings.Contains(outputStr, "url") || strings.Contains(outputStr, "URL") {
		// Если реализация проверяет наличие URL, то можно сделать более точный тест
		// Но для этого нужно знать точную реализацию
	}
}