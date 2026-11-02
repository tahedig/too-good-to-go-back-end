// Package validation holds small, dependency free input validators.
package validation

import (
	"errors"
	"regexp"
	"strings"
)

var (
	// ErrInvalidPhone is returned by NormalizePhone for unusable numbers.
	ErrInvalidPhone = errors.New("invalid phone number")

	e164      = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)
	irLocal   = regexp.MustCompile(`^09\d{9}$`)
	irIntl    = regexp.MustCompile(`^989\d{9}$`)
	separator = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "")
)

// NormalizePhone returns the number in E.164 form (+989123456789). It accepts
// E.164 input and Iranian local (0912...) or country-code (98912...) forms.
func NormalizePhone(raw string) (string, error) {
	p := separator.Replace(strings.TrimSpace(raw))
	switch {
	case irLocal.MatchString(p):
		p = "+98" + p[1:]
	case irIntl.MatchString(p):
		p = "+" + p
	case strings.HasPrefix(p, "00"):
		p = "+" + p[2:]
	}
	if !e164.MatchString(p) {
		return "", ErrInvalidPhone
	}
	return p, nil
}

// MaskPhone hides the middle digits for logs: +98*******6789.
func MaskPhone(p string) string {
	if len(p) <= 7 {
		return "***"
	}
	return p[:3] + strings.Repeat("*", len(p)-7) + p[len(p)-4:]
}
