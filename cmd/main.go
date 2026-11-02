// Package main — точка входа в утилиту `forumdump`.
//
// Реализует CLI-интерфейс на базе cobra: регистрирует флаги из
// internal/config (ТЗ 2.1), строит структурный логгер slog и запускает
// полный сценарий обхода темы в [pipeline].
package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/kirillshkro/gdumper/internal/config"
)

// exit* — коды завершения из ТЗ (раздел 6).
const (
	exitSuccess = 0 // Успех
	exitGeneric = 1 // Общая ошибка (сеть, парсинг)
	exitArgs    = 2 // Ошибка аргументов CLI
	exitDisk    = 3 // Ошибка записи на диск
	exitAuth    = 4 // Ошибка авторизации
	exitPartial = 5 // Частичный успех (есть предупреждения)
)

var (
	// rootCmd — корневая команда CLI (cobra).
	//
	// Флаги добавляются из config.NewFlagSet(); RunE загружает конфигурацию,
	// создаёт логгер slog и делегирует обход темы функции runPipeline.
	rootCmd = &cobra.Command{
		Use:   "forumdump",
		Short: "CLI-утилита для архивации тем форумов",
		Long:  `forumdump — CLI-утилита для архивации тем форумов.
Обходит тему форума по страницам, извлекает комментарии, вложенные ответы
и прикреплённые материалы, сохраняя результат в структурированном виде на диск.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Загрузка конфига покрывает все ошибки аргументов (ТЗ 2.1):
			// URL, формат, страницы, cookies, задержка, параллелизм, повторы.
			cfg, err := config.Load(cmd.Flags())
			if err != nil {
				// slog-атрибут error= даёт читательное сообщение в stderr.
				slog.Error("failed to load configuration", "error", err)
				return err
			}

			level := slog.LevelInfo
			if cfg.Verbose {
				level = slog.LevelDebug
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
			logger.Debug("configuration loaded", "format", cfg.Format, "out", cfg.OutputDir, "pages", cfg.Pages)

			// runPipeline возвращает код завершения; ненулевой — завершаем
			// процесс (основная точка выхода приложения).
			if code := runPipeline(cfg, logger); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
)

// main — точка входа: регистрирует флаги и запускает cobra.
//
// Execute возвращает ошибку только при невалидных аргументах (RunE), что
// соответствует коду завершения 2 из ТЗ.
func main() {
	rootCmd.Flags().AddFlagSet(config.NewFlagSet())
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitArgs)
	}
}
