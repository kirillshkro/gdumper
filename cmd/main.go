package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:   "forumdump",
		Short: "CLI-утилита для архивации тем форумов",
		Long: `forumdump — CLI-утилита для архивации тем форумов.
Обходит тему форума по страницам, извлекает комментарии, вложенные ответы и прикреплённые материалы,
сохраняя в структурированном виде на диск.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Проверяем, что указан URL
			if !cmd.Flags().Changed("url") {
				return fmt.Errorf("не указан URL темы форума (параметр --url)")
			}

			// Здесь должна быть логика выполнения основной задачи
			// Для тестирования пока возвращаем успех
			fmt.Println("Конфигурация загружена успешно")
			return nil
		},
	}
)

func init() {
	// Инициализация viper
	viper.SetEnvPrefix("FORUMDUMP")
	viper.AutomaticEnv()

	// Добавляем флаги
	rootCmd.Flags().String("url", "", "Ссылка на тему форума (абсолютный URL)")
	rootCmd.Flags().String("cookies", "", "Источник cookies: browser:chrome|firefox|edge|safari, file:/path/to/cookies.txt, raw:\"k=v; k2=v2\"")
	rootCmd.Flags().String("out", "", "Каталог вывода (по умолчанию ./dump_<timestamp>)")
	rootCmd.Flags().String("format", "json", "Формат вывода: json (по умолчанию), jsonl, markdown, html")
	rootCmd.Flags().String("pages", "all", "Диапазон страниц: all (по умолчанию), 1-5, 3, 3-")
	rootCmd.Flags().Int("concurrency", 1, "Число параллельных запросов (по умолчанию 1)")
	rootCmd.Flags().Int("delay", 500, "Задержка между запросами (по умолчанию 500ms)")
	rootCmd.Flags().String("attachments", "download", "Обработка вложений: download (по умолчанию), links, skip")
	rootCmd.Flags().Int("retries", 3, "Кол-во повторов при ошибках (по умолчанию 3)")
	rootCmd.Flags().String("user-agent", "", "Кастомный UA")
	rootCmd.Flags().String("config", "", "Путь к YAML/JSON-конфигу с правилами парсинга")
	rootCmd.Flags().Bool("verbose", false, "Подробный лог")

	// Привязываем флаги к viper
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if err := viper.BindPFlag(f.Name, f); err != nil {
			panic(fmt.Sprintf("ошибка привязки флага %s: %v", f.Name, err))
		}
	})
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2) // Exit code 2 для ошибок CLI аргументов
	}
}
