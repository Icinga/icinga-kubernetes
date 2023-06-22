package strcase

import (
	"strings"
	"unicode"
)

var separators = map[rune]interface{}{' ': struct{}{}, '_': struct{}{}, '-': struct{}{}, '.': struct{}{}}

func Snake(s string) string {
	return convert(s, unicode.LowerCase, '_')
}

func convert(s string, _case int, sep byte) string {
	if s == "" {
		return s
	}
	var wasLower bool

	s = strings.TrimSpace(s)

	n := strings.Builder{}
	n.Grow(len(s) + 2) // nominal 2 bytes of extra space for inserted delimiters

	for _, r := range []rune(s) {
		if _, ok := separators[r]; ok || wasLower && unicode.IsUpper(r) {
			n.WriteByte(sep)
		}

		n.WriteRune(unicode.To(_case, r))

		wasLower = unicode.IsLower(r)
	}

	return n.String()
}
