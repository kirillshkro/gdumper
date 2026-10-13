package config

import "errors"

var (
	// ErrNoOutputDir - ошибка отсутствия выходного каталога
	ErrNoOutputDir = errors.New("output directory not specified")

	// ErrInvalidFormat - ошибка невалидного формата
	ErrInvalidFormat = errors.New("invalid output format")
)

// Options представляет конфигурационные опции хранилища
type Options struct {
	OutputDir string `mapstructure:"output_dir" json:"output_dir"`
	Format    string `mapstructure:"format" json:"format"`
	Force     bool   `mapstructure:"force" json:"force"`
}

// Validate валидирует опции хранилища
func (o *Options) Validate() error {
	if o.OutputDir == "" {
		return ErrNoOutputDir
	}

	validFormats := map[string]bool{
		"json":     true,
		"jsonl":    true,
		"markdown": true,
		"html":     true,
	}

	if !validFormats[o.Format] {
		return ErrInvalidFormat
	}

	return nil
}

// NewOptions создает новые опции с дефолтными значениями
func NewOptions() *Options {
	return &Options{
		Format: "json",
	}
}