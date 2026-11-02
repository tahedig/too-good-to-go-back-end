package validation

// IsNumericCode reports whether code is exactly length ASCII digits.
func IsNumericCode(code string, length int) bool {
	if len(code) != length {
		return false
	}
	for i := 0; i < len(code); i++ {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}
