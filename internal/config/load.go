package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix - префикс переменных окружения (Description.md: ENV переменные)
const EnvPrefix = "FORUMDUMP"

// Имена флагов (ТЗ 2.1)
const (
	FlagURL         = "url"
	FlagCookies     = "cookies"
	FlagOutputDir   = "out"
	FlagFormat      = "format"
	FlagPages       = "pages"
	FlagConcurrency = "concurrency"
	FlagDelay       = "delay"
	FlagAttachments = "attachments"
	FlagRetries     = "retries"
	FlagUserAgent   = "user-agent"
	FlagConfig      = "config"
	FlagVerbose     = "verbose"
)

// NewFlagSet создает набор флагов со всеми флагами из ТЗ
func NewFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("forumdump", pflag.ContinueOnError)
	fs.String(FlagURL, "", "Ссылка на тему форума (абсолютный URL)")
	fs.String(FlagCookies, "", "Источник cookies: browser:chrome|firefox|edge|safari, file:/path/to/cookies.txt, raw:\"k=v; k2=v2\"")
	fs.String(FlagOutputDir, "", "Каталог вывода (по умолчанию ./dump_<timestamp>)")
	fs.String(FlagFormat, "json", "Формат вывода: json (по умолчанию), jsonl, markdown, html")
	fs.String(FlagPages, "all", "Диапазон страниц: all (по умолчанию), 1-5, 3, 3-")
	fs.Int(FlagConcurrency, 1, "Число параллельных запросов (по умолчанию 1)")
	fs.Int(FlagDelay, 500, "Задержка между запросами в мс (по умолчанию 500ms)")
	fs.String(FlagAttachments, "download", "Обработка вложений: download (по умолчанию), links, skip")
	fs.Int(FlagRetries, 3, "Кол-во повторов при ошибках (по умолчанию 3)")
	fs.String(FlagUserAgent, "", "Кастомный UA")
	fs.String(FlagConfig, "", "Путь к YAML/JSON-конфигу с правилами парсинга")
	fs.Bool(FlagVerbose, false, "Подробный лог")
	return fs
}

// Load загружает конфигурацию приложения.
// Приоритет: флаги командной строки > переменные окружения >
// конфигурационный файл > значения по умолчанию (Description.md).
func Load(flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	setDefaults(v, NewConfig())

	if err := bindFlags(v, flags); err != nil {
		return nil, err
	}

	if err := loadConfigFile(v, flags); err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// setDefaults выставляет значения по умолчанию для всех параметров в viper
func setDefaults(v *viper.Viper, defaults *Config) {
	v.SetDefault(FlagURL, defaults.URL)
	v.SetDefault(FlagCookies, defaults.Cookies)
	v.SetDefault(FlagOutputDir, defaults.OutputDir)
	v.SetDefault(FlagFormat, defaults.Format)
	v.SetDefault(FlagPages, defaults.Pages)
	v.SetDefault(FlagConcurrency, defaults.Concurrency)
	v.SetDefault(FlagDelay, defaults.DelayMS)
	v.SetDefault(FlagAttachments, defaults.Attachments)
	v.SetDefault(FlagRetries, defaults.Retries)
	v.SetDefault(FlagUserAgent, defaults.UserAgent)
	v.SetDefault(FlagConfig, defaults.ConfigFile)
	v.SetDefault(FlagVerbose, defaults.Verbose)
}

// bindFlags привязывает все флаги pflag к экземпляру viper
func bindFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	var bindErr error
	flags.VisitAll(func(f *pflag.Flag) {
		if bindErr == nil {
			bindErr = v.BindPFlag(f.Name, f)
		}
	})
	if bindErr != nil {
		return fmt.Errorf("failed to bind flags: %w", bindErr)
	}
	return nil
}

// loadConfigFile загружает YAML/JSON-файл конфигурации, если задан флаг --config
func loadConfigFile(v *viper.Viper, flags *pflag.FlagSet) error {
	if flags.Lookup(FlagConfig) == nil {
		return nil
	}
	path, err := flags.GetString(FlagConfig)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file %q: %w", path, err)
	}
	slog.Debug("config file loaded", "path", path)
	return nil
}
