package logger

import "testing"

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with query params strips them",
			input:    "https://sonarr.example.com/api/v3/series?apikey=secret123",
			expected: "https://sonarr.example.com/api/v3/series",
		},
		{
			name:     "URL with multiple query params",
			input:    "https://example.com/api?key=abc&token=xyz&foo=bar",
			expected: "https://example.com/api",
		},
		{
			name:     "URL without query params returns unchanged",
			input:    "https://example.com/api/v3/series",
			expected: "https://example.com/api/v3/series",
		},
		{
			name:     "URL with fragment strips it",
			input:    "https://example.com/page#section",
			expected: "https://example.com/page",
		},
		{
			name:     "URL with query and fragment strips both",
			input:    "https://example.com/api?key=secret#frag",
			expected: "https://example.com/api",
		},
		{
			name:     "Empty string returns empty",
			input:    "",
			expected: "",
		},
		{
			name:     "Plain path without scheme",
			input:    "/api/v3/series",
			expected: "/api/v3/series",
		},
		{
			name:     "Malformed URL returns invalid-url sentinel",
			input:    "://not-a-url",
			expected: "[invalid-url]",
		},
		{
			name:     "URL with port number preserved",
			input:    "http://localhost:8989/api/v3/system?apikey=abc",
			expected: "http://localhost:8989/api/v3/system",
		},
		{
			name:     "URL with trailing slash",
			input:    "https://example.com/api/?token=secret",
			expected: "https://example.com/api/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeURL(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeURL(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
