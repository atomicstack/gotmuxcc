package gotmuxcc

import (
	"strconv"
	"strings"
)

func checkSessionName(name string) bool {
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, ":.\n\r\x00") {
		return false
	}
	return true
}

func isOne(value string) bool {
	return value == "1"
}

func parseList(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

// findBy returns the first element of list satisfying match, or nil when none
// does. The gotmux-compatible API reports "not found" as a (nil, nil) result,
// so callers return the nil element alongside a nil error.
func findBy[T any](list []*T, match func(*T) bool) *T {
	for _, item := range list {
		if match(item) {
			return item
		}
	}
	return nil
}

func atoi(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func atoi32(value string) int32 {
	return int32(atoi(value))
}
