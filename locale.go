package main

import (
	"os"
	"strings"
)

func detectSystemLocale() string {
	envVars := []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"}
	for _, name := range envVars {
		if lang := normalizeLocale(os.Getenv(name)); lang != "" {
			return lang
		}
	}
	if lang := detectLocaleWindows(); lang != "" {
		return lang
	}
	return "en"
}

func normalizeLocale(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	val = strings.ToLower(val)
	separators := []string{".", "_", "-"}
	for _, sep := range separators {
		if idx := strings.Index(val, sep); idx >= 0 {
			val = val[:idx]
		}
	}
	switch {
	case strings.HasPrefix(val, "ru"):
		return "ru"
	case strings.HasPrefix(val, "en"):
		return "en"
	default:
		return ""
	}
}
