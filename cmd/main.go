package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/kirillshkro/gdumper/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "forumdump",
	Short: "CLI-утилита для архивации тем форумов",
	Long: `forumdump — CLI-утилита для архивации тем форумов.
Обходит тему форума по страницам, извлекает комментарии, вложенные ответы и прикреплённые материалы,
сохраняя в структурированном виде на диск.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(cmd.Flags())
		if err != nil {
			slog.Error("failed to load configuration", "error", err)
			return err
		}
		level := slog.LevelInfo
		if cfg.Verbose {
			level = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
		logger.Info("configuration loaded", "format", cfg.Format, "out", cfg.OutputDir, "pages", cfg.Pages)
		return nil
	},
}

func main() {
	rootCmd.Flags().AddFlagSet(config.NewFlagSet())
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2) // Exit code 2 - ошибка аргументов CLI (ТЗ раздел 6)
	}
}
