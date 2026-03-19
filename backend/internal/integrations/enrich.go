package integrations

import "strings"

// normalizedTitleKey returns the lowercase title key for matching.
// For season items with a ShowTitle, uses the show title instead.
func normalizedTitleKey(item *MediaItem) string {
	titleKey := strings.ToLower(strings.TrimSpace(item.Title))
	if item.ShowTitle != "" {
		titleKey = strings.ToLower(strings.TrimSpace(item.ShowTitle))
	}
	return titleKey
}
