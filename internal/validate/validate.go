// Package validate valida entradas vindas da interface (contracts/bridge.md: "toda entrada é
// validada no Go"). Toda falha vira INVALID_INPUT.
package validate

import (
	"strings"
	"unicode/utf8"

	"openterminal/internal/apperr"
)

func invalid() error { return apperr.New(apperr.InvalidInput) }

// UUID aceita o formato canônico 8-4-4-4-12 em hexadecimal.
func UUID(s string) error {
	if len(s) != 36 {
		return invalid()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return invalid()
			}
		default:
			if !isHex(c) {
				return invalid()
			}
		}
	}
	return nil
}

func OptionalUUID(s *string) error {
	if s == nil {
		return nil
	}
	return UUID(*s)
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// Text apara espaços e exige entre min e max caracteres, sem caracteres de controle.
func Text(s string, min, max int) (string, error) {
	t := strings.TrimSpace(s)
	if !utf8.ValidString(t) {
		return "", invalid()
	}
	n := utf8.RuneCountInString(t)
	if n < min || n > max {
		return "", invalid()
	}
	for _, r := range t {
		if r < 0x20 || r == 0x7f {
			return "", invalid()
		}
	}
	return t, nil
}

func Int(n, min, max int) error {
	if n < min || n > max {
		return invalid()
	}
	return nil
}

func OneOf[T comparable](v T, allowed ...T) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return invalid()
}
