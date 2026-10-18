package config_test

import (
	"github.com/kirillshkro/gdumper/internal/config"
)

// TestParseCookiesEmpty проверяет пустой источник cookies
func (s *ConfigTestSuite) TestParseCookiesEmpty() {
	src, err := config.ParseCookies("")
	s.NoError(err)
	s.Nil(src)
}

// TestParseCookiesBrowser проверяет источники вида "browser:<имя>"
func (s *ConfigTestSuite) TestParseCookiesBrowser() {
	for _, browser := range []string{"chrome", "firefox", "edge", "safari"} {
		src, err := config.ParseCookies("browser:" + browser)
		s.Require().NoError(err)
		s.Equal(config.CookieKindBrowser, src.Kind)
		s.Equal(browser, src.Browser)
	}
	_, err := config.ParseCookies("browser:ie")
	s.ErrorIs(err, config.ErrUnknownBrowser)
}

// TestParseCookiesFile проверяет источники вида "file:<путь>"
func (s *ConfigTestSuite) TestParseCookiesFile() {
	src, err := config.ParseCookies("file:/tmp/cookies.txt")
	s.Require().NoError(err)
	s.Equal(config.CookieKindFile, src.Kind)
	s.Equal("/tmp/cookies.txt", src.Path)

	_, err = config.ParseCookies("file:")
	s.ErrorIs(err, config.ErrEmptyCookieSource)
}

// TestParseCookiesRaw проверяет источники вида "raw:\"k=v; k2=v2\""
func (s *ConfigTestSuite) TestParseCookiesRaw() {
	src, err := config.ParseCookies(`raw:"k=v; k2=v2"`)
	s.Require().NoError(err)
	s.Equal(config.CookieKindRaw, src.Kind)
	s.Equal([]string{"k=v", "k2=v2"}, src.Pairs)

	_, err = config.ParseCookies(`raw:""`)
	s.ErrorIs(err, config.ErrEmptyCookieSource)

	_, err = config.ParseCookies(`raw:"novalue"`)
	s.ErrorIs(err, config.ErrInvalidCookiePair)
}

// TestParseCookiesUnknownKind проверяет источники неизвестного вида
func (s *ConfigTestSuite) TestParseCookiesUnknownKind() {
	_, err := config.ParseCookies("weird:source")
	s.ErrorIs(err, config.ErrUnknownCookieKind)

	_, err = config.ParseCookies("withoutcolon")
	s.ErrorIs(err, config.ErrUnknownCookieKind)
}