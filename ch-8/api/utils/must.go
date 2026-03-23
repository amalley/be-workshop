package utils

import "strconv"

// MustParseInt is a helper function that parses a string into an integer and panics if it fails.
func MustParseInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic("failed to parse int: " + err.Error())
	}
	return v
}
