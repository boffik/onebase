// Package i18nerr — ошибки платформы с локализуемым сообщением.
//
// Ключ — русский fmt-шаблон («неизвестная таблица %s»), как и все ключи
// i18n OneBase. Error() всегда рендерит по-русски (логи, CLI не меняются),
// Localize переводит сообщение на HTTP-границе: i18nerr-звенья цепочки —
// по шаблону с подстановкой аргументов, прочие ошибки — exact-match-ом
// полного текста; всё, что перевести нечем, остаётся русским.
package i18nerr

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ivantit66/onebase/internal/i18n"
)

// Error — ошибка с шаблоном-ключом и аргументами.
type Error struct {
	Key     string
	Args    []any
	wrapped error
}

// New создаёт ошибку со статическим ключом.
func New(key string) error { return &Error{Key: key} }

// Errorf создаёт ошибку с fmt-шаблоном и аргументами.
func Errorf(key string, args ...any) error { return &Error{Key: key, Args: args} }

// Wrapf оборачивает err локализуемым префиксом: «<key с args>: <err>».
func Wrapf(err error, key string, args ...any) error {
	return &Error{Key: key, Args: args, wrapped: err}
}

func (e *Error) Error() string {
	msg := e.render()
	if e.wrapped != nil {
		return msg + ": " + e.wrapped.Error()
	}
	return msg
}

func (e *Error) Unwrap() error { return e.wrapped }

// render — русское сообщение без wrapped-части.
func (e *Error) render() string {
	if len(e.Args) == 0 {
		return e.Key
	}
	return fmt.Sprintf(e.Key, e.Args...)
}

// localize — перевод шаблона и подстановка аргументов.
func (e *Error) localize(b *i18n.Bundle, lang string) string {
	tpl := b.T(lang, e.Key)
	if len(e.Args) == 0 {
		return tpl
	}
	return fmt.Sprintf(tpl, e.Args...)
}

// Localize переводит сообщение об ошибке для языка lang.
func Localize(b *i18n.Bundle, lang string, err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if b == nil || lang == "" || lang == "ru" {
		return msg
	}
	// Статическое сообщение целиком (включая ошибки без i18nerr).
	if t := b.T(lang, msg); t != msg {
		return t
	}
	// Перевести i18nerr-звенья в цепочке, сохранив остальной текст.
	for c := err; c != nil; c = errors.Unwrap(c) {
		if e, ok := c.(*Error); ok {
			ru := e.render()
			if loc := e.localize(b, lang); loc != ru {
				msg = strings.Replace(msg, ru, loc, 1)
			}
		}
	}
	return msg
}
