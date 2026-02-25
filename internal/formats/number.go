package formats

// IsWholeNumber reports whether f represents an integer value (no fractional part).
func IsWholeNumber(f float64) bool {
	return f == float64(int64(f))
}
