package handlers

import (
	"fmt"
	"strconv"
)

// itoa is a convenience wrapper for int to string conversion.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// scanInt parses s as a base-10 integer, returning an error if it fails.
func scanInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.Atoi(s)
}
