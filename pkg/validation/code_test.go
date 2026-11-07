package validation

import "testing"

func TestIsNumericCode(t *testing.T) {
	cases := []struct {
		code   string
		length int
		want   bool
	}{
		{"123456", 6, true},
		{"000000", 6, true},
		{"12345", 6, false},
		{"1234567", 6, false},
		{"12a456", 6, false},
		{"١٢٣٤٥٦", 6, false}, // Arabic-Indic digits are rejected
		{"", 6, false},
	}
	for _, tc := range cases {
		if got := IsNumericCode(tc.code, tc.length); got != tc.want {
			t.Errorf("IsNumericCode(%q, %d) = %v, want %v", tc.code, tc.length, got, tc.want)
		}
	}
}
