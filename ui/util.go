package ui

import "strings"

// cls joins class fragments, skipping empty ones. elem-go escapes attribute
// values itself, so nothing here escapes (escaping twice would corrupt them).
func cls(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}

	return strings.Join(out, " ")
}

// onClass returns class when cond holds, for cls("a", onClass(x, "b")).
func onClass(cond bool, class string) string {
	if cond {
		return class
	}

	return ""
}
