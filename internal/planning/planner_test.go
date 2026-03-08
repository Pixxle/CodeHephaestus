package planning

import "testing"

func TestParseQuestions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "no questions section",
			input: "## Analysis\nLooks good.",
			want:  nil,
		},
		{
			name: "single question",
			input: `### Open Questions
1. What database should we use?
`,
			want: []string{"What database should we use?"},
		},
		{
			name: "multiple questions",
			input: `### Open Questions
1. What database should we use?
2. Should we add caching?
3. What is the deployment target?
`,
			want: []string{
				"What database should we use?",
				"Should we add caching?",
				"What is the deployment target?",
			},
		},
		{
			name: "questions section followed by another section",
			input: `### Open Questions
1. First question?

### Next Steps
Do something.
`,
			want: []string{"First question?"},
		},
		{
			name:  "empty questions section",
			input: "### Open Questions\n\n### Next Steps\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseQuestions(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseQuestions() returned %d questions, want %d\ngot: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("question[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestContainsReadyKeyword(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"ready to go", true},
		{"LGTM", true},
		{"approved", true},
		{"Go ahead and build", true},
		{"Looks good to me", true},
		{"Start building", true},
		{"I have a question", false},
		{"not sure about this", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := containsReadyKeyword(tt.text); got != tt.want {
				t.Errorf("containsReadyKeyword(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestDescriptionChanged(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		lastSeen string
		want     bool
	}{
		{"same content", "hello", "hello", false},
		{"different content", "hello", "world", true},
		{"whitespace only diff", "  hello  ", "hello", false},
		{"both empty", "", "", false},
		{"empty vs content", "", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DescriptionChanged(tt.current, tt.lastSeen); got != tt.want {
				t.Errorf("DescriptionChanged(%q, %q) = %v, want %v", tt.current, tt.lastSeen, got, tt.want)
			}
		})
	}
}

func TestEnsureCorrectHeading(t *testing.T) {
	botName := "TestBot"
	tests := []struct {
		name        string
		input       string
		noQuestions bool
		want        string
	}{
		{
			name:        "upgrade to complete when no questions",
			input:       "## TestBot — Planning\nSome content",
			noQuestions: true,
			want:        "## TestBot — Planning Complete\nSome content",
		},
		{
			name:        "downgrade to planning when has questions",
			input:       "## TestBot — Planning Complete\nSome content",
			noQuestions: false,
			want:        "## TestBot — Planning\nSome content",
		},
		{
			name:        "already correct complete heading",
			input:       "## TestBot — Planning Complete\nSome content",
			noQuestions: true,
			want:        "## TestBot — Planning Complete\nSome content",
		},
		{
			name:        "already correct planning heading",
			input:       "## TestBot — Planning\nSome content",
			noQuestions: false,
			want:        "## TestBot — Planning\nSome content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureCorrectHeading(tt.input, tt.noQuestions, botName)
			if got != tt.want {
				t.Errorf("ensureCorrectHeading() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsImageMime(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/gif", true},
		{"image/svg+xml", true},
		{"image/webp", true},
		{"application/pdf", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			if got := isImageMime(tt.mime); got != tt.want {
				t.Errorf("isImageMime(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}
