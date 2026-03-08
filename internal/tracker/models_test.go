package tracker

import (
	"strings"
	"testing"
	"time"
)

func TestIsAssignedTo(t *testing.T) {
	issue := Issue{
		Key:       "TEST-1",
		Assignees: []string{"user-123", "user-456"},
	}

	if !issue.IsAssignedTo("user-123") {
		t.Error("expected IsAssignedTo to return true for user-123")
	}
	if !issue.IsAssignedTo("user-456") {
		t.Error("expected IsAssignedTo to return true for user-456")
	}
	if issue.IsAssignedTo("user-999") {
		t.Error("expected IsAssignedTo to return false for user-999")
	}

	empty := Issue{Key: "TEST-2"}
	if empty.IsAssignedTo("user-123") {
		t.Error("expected IsAssignedTo to return false for nil assignees")
	}
}

func TestFormatConversation(t *testing.T) {
	now := time.Now()
	comments := []Comment{
		{ID: "1", Author: "user1", Body: "Hello", Created: now},
		{ID: "2", Author: "bot-id", Body: "Hi there", Created: now},
		{ID: "3", Author: "user2", Body: "Question?", Created: now},
	}

	t.Run("with bot user ID", func(t *testing.T) {
		got := FormatConversation(comments, "bot-id")
		if !strings.Contains(got, "[human] user1:") {
			t.Error("expected [human] prefix for user1")
		}
		if !strings.Contains(got, "[assistant] bot-id:") {
			t.Error("expected [assistant] prefix for bot")
		}
		if !strings.Contains(got, "[human] user2:") {
			t.Error("expected [human] prefix for user2")
		}
		if !strings.Contains(got, "Hello") || !strings.Contains(got, "Hi there") || !strings.Contains(got, "Question?") {
			t.Error("expected all comment bodies in output")
		}
	})

	t.Run("without bot user ID", func(t *testing.T) {
		got := FormatConversation(comments, "")
		if strings.Contains(got, "[human]") || strings.Contains(got, "[assistant]") {
			t.Error("should not have role prefixes when botUserID is empty")
		}
		if !strings.Contains(got, "[user1]:") {
			t.Error("expected [user1]: prefix")
		}
		if !strings.Contains(got, "[bot-id]:") {
			t.Error("expected [bot-id]: prefix")
		}
	})

	t.Run("empty comments", func(t *testing.T) {
		got := FormatConversation(nil, "bot-id")
		if got != "" {
			t.Errorf("expected empty string for nil comments, got %q", got)
		}
	})
}
