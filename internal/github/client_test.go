package github

import "testing"

func TestParsePRNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "valid JSON with PR",
			input: `[{"number": 42}]`,
			want:  42,
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  0,
		},
		{
			name:  "invalid JSON",
			input: `not json`,
			want:  0,
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "multiple PRs returns first",
			input: `[{"number": 10}, {"number": 20}]`,
			want:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePRNumber(tt.input); got != tt.want {
				t.Errorf("parsePRNumber(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseReactedComments(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		check   func(t *testing.T, comments []PRComment)
	}{
		{
			name:    "empty string",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "invalid JSON",
			input:   "not json",
			wantLen: 0,
		},
		{
			name: "comment with thumbs up",
			input: `[{
				"id": 1,
				"body": "LGTM",
				"created_at": "2024-01-01T00:00:00Z",
				"user": {"login": "reviewer"},
				"reactions": {"+1": 1, "eyes": 0}
			}]`,
			wantLen: 1,
			check: func(t *testing.T, comments []PRComment) {
				if comments[0].Reaction != "thumbs_up" {
					t.Errorf("Reaction = %q, want %q", comments[0].Reaction, "thumbs_up")
				}
				if comments[0].Author != "reviewer" {
					t.Errorf("Author = %q, want %q", comments[0].Author, "reviewer")
				}
			},
		},
		{
			name: "comment with eyes reaction",
			input: `[{
				"id": 2,
				"body": "Looking at this",
				"created_at": "2024-01-01T00:00:00Z",
				"user": {"login": "dev"},
				"reactions": {"+1": 0, "eyes": 1}
			}]`,
			wantLen: 1,
			check: func(t *testing.T, comments []PRComment) {
				if comments[0].Reaction != "eyes" {
					t.Errorf("Reaction = %q, want %q", comments[0].Reaction, "eyes")
				}
			},
		},
		{
			name: "comment with no reactions filtered out",
			input: `[{
				"id": 3,
				"body": "Just a comment",
				"created_at": "2024-01-01T00:00:00Z",
				"user": {"login": "dev"},
				"reactions": {"+1": 0, "eyes": 0}
			}]`,
			wantLen: 0,
		},
		{
			name: "mixed reactions",
			input: `[
				{
					"id": 1,
					"body": "LGTM",
					"created_at": "2024-01-01T00:00:00Z",
					"user": {"login": "a"},
					"reactions": {"+1": 2, "eyes": 0}
				},
				{
					"id": 2,
					"body": "no reaction",
					"created_at": "2024-01-01T00:00:00Z",
					"user": {"login": "b"},
					"reactions": {"+1": 0, "eyes": 0}
				},
				{
					"id": 3,
					"body": "watching",
					"created_at": "2024-01-01T00:00:00Z",
					"user": {"login": "c"},
					"reactions": {"+1": 0, "eyes": 1}
				}
			]`,
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseReactedComments(tt.input, "review_comment")
			if len(got) != tt.wantLen {
				t.Fatalf("parseReactedComments() returned %d comments, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
