package plugin

import (
	"github.com/pixxle/solomon/internal/slack"
)

// NewNotifier creates a plugin-specific Slack notifier using the plugin's
// slack_channel_id setting (falling back to the global channel). Returns a
// NoopNotifier when no Slack client is configured.
func NewNotifier(libs *SharedLibs, settings map[string]interface{}) slack.Notifier {
	if libs.SlackClient == nil {
		return &slack.NoopNotifier{}
	}
	channelID := SettingString(settings, "slack_channel_id", libs.Config.SlackChannelID)
	return slack.NewSlackNotifier(libs.SlackClient, channelID, libs.DB)
}

// SettingString extracts a string value from a plugin settings map,
// returning fallback if the key is missing or empty.
func SettingString(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

// SettingBool extracts a bool value from a plugin settings map.
// Supports both bool and "true"/"false" string representations.
func SettingBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		switch b := v.(type) {
		case bool:
			return b
		case string:
			return b == "true"
		}
	}
	return false
}

// SettingStringSlice extracts a string slice from a plugin settings map.
// Handles YAML list → []interface{} → []string conversion.
// Returns fallback if key missing or empty.
func SettingStringSlice(m map[string]interface{}, key string, fallback []string) []string {
	v, ok := m[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case []interface{}:
		if len(val) == 0 {
			return fallback
		}
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		if len(val) == 0 {
			return fallback
		}
		return val
	}
	return fallback
}

// SettingInt extracts an int value from a plugin settings map,
// returning fallback if the key is missing. Handles both int and
// float64 (as produced by YAML/JSON unmarshaling).
func SettingInt(m map[string]interface{}, key string, fallback int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return fallback
}
