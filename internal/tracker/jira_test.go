package tracker

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Fix Bug #123", "fix-bug-123"},
		{"UPPER CASE TITLE", "upper-case-title"},
		{"already-slugified", "already-slugified"},
		{"Special $chars! @here", "special-chars-here"},
		{"", ""},
		{"A very long title that exceeds the fifty character maximum length for slugs", "a-very-long-title-that-exceeds-the-fifty-character"},
		{"trailing---dashes---", "trailing-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := slugify(tt.input); got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseHeading(t *testing.T) {
	tests := []struct {
		line      string
		wantLevel int
		wantText  string
	}{
		{"# Heading 1", 1, "Heading 1"},
		{"## Heading 2", 2, "Heading 2"},
		{"### Heading 3", 3, "Heading 3"},
		{"###### Heading 6", 6, "Heading 6"},
		{"Not a heading", 0, ""},
		{"#NoSpace", 0, ""},
		{"", 0, ""},
		{"####### Too many hashes", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			level, text := parseHeading(tt.line)
			if level != tt.wantLevel || text != tt.wantText {
				t.Errorf("parseHeading(%q) = (%d, %q), want (%d, %q)",
					tt.line, level, text, tt.wantLevel, tt.wantText)
			}
		})
	}
}

func TestTextToADF(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"plain paragraph", "Hello world"},
		{"heading", "## My Heading"},
		{"bullet list", "- item one\n- item two"},
		{"numbered list", "1. first\n2. second"},
		{"horizontal rule", "---"},
		{"task list", "- [ ] todo\n- [x] done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textToADF(tt.input)
			if result["type"] != "doc" {
				t.Error("textToADF() type != doc")
			}
			if result["version"] != 1 {
				t.Error("textToADF() version != 1")
			}
			content, ok := result["content"].([]interface{})
			if !ok || len(content) == 0 {
				t.Error("textToADF() has no content")
			}
		})
	}
}

func TestExtractADFText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "null input",
			input: "null",
			want:  "",
		},
		{
			name:  "plain string",
			input: `"hello world"`,
			want:  "hello world",
		},
		{
			name: "simple ADF document",
			input: `{
				"type": "doc",
				"version": 1,
				"content": [
					{
						"type": "paragraph",
						"content": [
							{"type": "text", "text": "Hello "},
							{"type": "text", "text": "World"}
						]
					}
				]
			}`,
			want: "Hello World",
		},
		{
			name: "multi-paragraph ADF",
			input: `{
				"type": "doc",
				"version": 1,
				"content": [
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "First"}]
					},
					{
						"type": "paragraph",
						"content": [{"type": "text", "text": "Second"}]
					}
				]
			}`,
			want: "First\nSecond",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractADFText(json.RawMessage(tt.input))
			if got != tt.want {
				t.Errorf("extractADFText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJiraTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "standard Jira format",
			input: "2024-01-15T10:30:00.000+0000",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "UTC Z format",
			input: "2024-01-15T10:30:00.000Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "RFC3339",
			input: "2024-01-15T10:30:00Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "invalid format",
			input: "not a time",
			want:  time.Time{},
		},
		{
			name:  "empty string",
			input: "",
			want:  time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJiraTime(tt.input)
			if !got.Equal(tt.want) {
				t.Errorf("parseJiraTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTaskItem(t *testing.T) {
	tests := []struct {
		input      string
		wantCheck  bool
		wantIsTask bool
	}{
		{"[ ] unchecked", false, true},
		{"[x] checked", true, true},
		{"[X] checked upper", true, true},
		{"regular text", false, false},
		{"[ ]no space", false, true},
		{"[x]no space", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			checked, isTask := parseTaskItem(tt.input)
			if checked != tt.wantCheck || isTask != tt.wantIsTask {
				t.Errorf("parseTaskItem(%q) = (%v, %v), want (%v, %v)",
					tt.input, checked, isTask, tt.wantCheck, tt.wantIsTask)
			}
		})
	}
}

func TestSplitTableRow(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"| a | b | c |", 3},
		{"| single |", 1},
		{"| one | two |", 2},
		{"|---|---|", 2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitTableRow(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitTableRow(%q) returned %d cells, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestGetIssueBranchName(t *testing.T) {
	j := &JiraTracker{}
	issue := Issue{Key: "PROJ-123", Title: "Add user login"}
	got := j.GetIssueBranchName(issue, "mybot")
	want := "mybot/PROJ-123-add-user-login"
	if got != want {
		t.Errorf("GetIssueBranchName() = %q, want %q", got, want)
	}
}
