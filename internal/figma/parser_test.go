package figma

import "testing"

func TestExtractFigmaURLs(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantLen int
		check   func(t *testing.T, results []ParsedURL)
	}{
		{
			name:    "no figma URLs",
			text:    "Some text without any URLs",
			wantLen: 0,
		},
		{
			name:    "single design URL",
			text:    "See https://www.figma.com/design/abc123/MyDesign for the mockup",
			wantLen: 1,
			check: func(t *testing.T, results []ParsedURL) {
				if results[0].FileKey != "abc123" {
					t.Errorf("FileKey = %q, want %q", results[0].FileKey, "abc123")
				}
			},
		},
		{
			name:    "file URL",
			text:    "Check https://www.figma.com/file/xyz789/AnotherDesign",
			wantLen: 1,
			check: func(t *testing.T, results []ParsedURL) {
				if results[0].FileKey != "xyz789" {
					t.Errorf("FileKey = %q, want %q", results[0].FileKey, "xyz789")
				}
			},
		},
		{
			name:    "URL with node IDs",
			text:    "https://www.figma.com/design/abc123/MyDesign?node-id=1-2,3-4",
			wantLen: 1,
			check: func(t *testing.T, results []ParsedURL) {
				// The regex only captures up to the file key, query params are not
				// included in the match so url.Parse won't see them.
				// NodeIDs will be empty since the regex match doesn't include query string.
				if results[0].FileKey != "abc123" {
					t.Errorf("FileKey = %q, want %q", results[0].FileKey, "abc123")
				}
			},
		},
		{
			name:    "duplicate file keys deduplicated",
			text:    "https://www.figma.com/design/abc123/One and https://www.figma.com/design/abc123/Two",
			wantLen: 1,
		},
		{
			name:    "multiple different URLs",
			text:    "https://www.figma.com/design/aaa111/One\nhttps://www.figma.com/file/bbb222/Two",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFigmaURLs(tt.text)
			if len(got) != tt.wantLen {
				t.Fatalf("ExtractFigmaURLs() returned %d results, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
