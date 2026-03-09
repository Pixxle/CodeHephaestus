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

func TestParseComments(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		commentType string
		wantLen     int
		check       func(t *testing.T, comments []PRComment)
	}{
		{
			name:        "empty string",
			input:       "",
			commentType: "review_comment",
			wantLen:     0,
		},
		{
			name:        "invalid JSON",
			input:       "not json",
			commentType: "review_comment",
			wantLen:     0,
		},
		{
			name: "review comment with thumbs up populates NodeID and Type",
			input: `[{
				"id": 1,
				"node_id": "PRRC_abc123",
				"body": "LGTM",
				"created_at": "2024-01-01T00:00:00Z",
				"user": {"login": "reviewer"},
				"reactions": {"+1": 1, "eyes": 0}
			}]`,
			commentType: "review_comment",
			wantLen:     1,
			check: func(t *testing.T, comments []PRComment) {
				if comments[0].Reaction != "thumbs_up" {
					t.Errorf("Reaction = %q, want %q", comments[0].Reaction, "thumbs_up")
				}
				if comments[0].Author != "reviewer" {
					t.Errorf("Author = %q, want %q", comments[0].Author, "reviewer")
				}
				if comments[0].NodeID != "PRRC_abc123" {
					t.Errorf("NodeID = %q, want %q", comments[0].NodeID, "PRRC_abc123")
				}
				if comments[0].Type != "review_comment" {
					t.Errorf("Type = %q, want %q", comments[0].Type, "review_comment")
				}
			},
		},
		{
			name: "issue comment preserves type",
			input: `[{
				"id": 10,
				"node_id": "IC_xyz789",
				"body": "Question about the approach",
				"created_at": "2024-01-02T00:00:00Z",
				"user": {"login": "dev"},
				"reactions": {"+1": 0, "eyes": 1}
			}]`,
			commentType: "issue_comment",
			wantLen:     1,
			check: func(t *testing.T, comments []PRComment) {
				if comments[0].Reaction != "eyes" {
					t.Errorf("Reaction = %q, want %q", comments[0].Reaction, "eyes")
				}
				if comments[0].Type != "issue_comment" {
					t.Errorf("Type = %q, want %q", comments[0].Type, "issue_comment")
				}
				if comments[0].NodeID != "IC_xyz789" {
					t.Errorf("NodeID = %q, want %q", comments[0].NodeID, "IC_xyz789")
				}
			},
		},
		{
			name: "comment with eyes reaction",
			input: `[{
				"id": 2,
				"node_id": "PRRC_def456",
				"body": "Looking at this",
				"created_at": "2024-01-01T00:00:00Z",
				"user": {"login": "dev"},
				"reactions": {"+1": 0, "eyes": 1}
			}]`,
			commentType: "review_comment",
			wantLen:     1,
			check: func(t *testing.T, comments []PRComment) {
				if comments[0].Reaction != "eyes" {
					t.Errorf("Reaction = %q, want %q", comments[0].Reaction, "eyes")
				}
				if comments[0].NodeID != "PRRC_def456" {
					t.Errorf("NodeID = %q, want %q", comments[0].NodeID, "PRRC_def456")
				}
			},
		},
		{
			name: "comment with no reactions included with empty reaction",
			input: `[{
				"id": 3,
				"node_id": "PRRC_ghi",
				"body": "Just a comment",
				"created_at": "2024-01-01T00:00:00Z",
				"user": {"login": "dev"},
				"reactions": {"+1": 0, "eyes": 0}
			}]`,
			commentType: "review_comment",
			wantLen:     1,
			check: func(t *testing.T, comments []PRComment) {
				if comments[0].Reaction != "" {
					t.Errorf("Reaction = %q, want empty string", comments[0].Reaction)
				}
			},
		},
		{
			name: "mixed reactions",
			input: `[
				{
					"id": 1,
					"node_id": "PRRC_1",
					"body": "LGTM",
					"created_at": "2024-01-01T00:00:00Z",
					"user": {"login": "a"},
					"reactions": {"+1": 2, "eyes": 0}
				},
				{
					"id": 2,
					"node_id": "PRRC_2",
					"body": "no reaction",
					"created_at": "2024-01-01T00:00:00Z",
					"user": {"login": "b"},
					"reactions": {"+1": 0, "eyes": 0}
				},
				{
					"id": 3,
					"node_id": "PRRC_3",
					"body": "watching",
					"created_at": "2024-01-01T00:00:00Z",
					"user": {"login": "c"},
					"reactions": {"+1": 0, "eyes": 1}
				}
			]`,
			commentType: "review_comment",
			wantLen:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseComments(tt.input, tt.commentType)
			if len(got) != tt.wantLen {
				t.Fatalf("parseComments() returned %d comments, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
