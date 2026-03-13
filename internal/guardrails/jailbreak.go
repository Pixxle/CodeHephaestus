package guardrails

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/pixxle/solomon/internal/claude"
)

// ScanResult holds the result of a jailbreak scan.
type ScanResult struct {
	Blocked bool   `json:"blocked"`
	Reason  string `json:"reason"`
}

// ScanForJailbreak checks user-provided content for prompt injection or jailbreak attempts.
// It uses a quick LLM call to classify the content. The function is fail-open: on any LLM
// or parse error it logs a warning and returns a non-blocked result to avoid disrupting
// legitimate work.
func ScanForJailbreak(ctx context.Context, content, source, model string) *ScanResult {
	if strings.TrimSpace(content) == "" {
		return &ScanResult{Blocked: false}
	}

	prompt, err := claude.RenderPrompt("jailbreak_scan.md.tmpl", map[string]interface{}{
		"Source":  source,
		"Content": content,
	})
	if err != nil {
		log.Warn().Err(err).Str("source", source).Msg("failed to render jailbreak scan prompt, allowing content through")
		return &ScanResult{Blocked: false}
	}

	output, err := claude.RunClaudeQuick(ctx, prompt, model)
	if err != nil {
		log.Warn().Err(err).Str("source", source).Msg("jailbreak scan failed, allowing content through")
		return &ScanResult{Blocked: false}
	}

	var result ScanResult
	if err := json.Unmarshal([]byte(claude.StripCodeFence(output)), &result); err != nil {
		log.Warn().Err(err).Str("output", output).Str("source", source).Msg("failed to parse jailbreak scan result, allowing content through")
		return &ScanResult{Blocked: false}
	}

	if result.Blocked {
		log.Warn().
			Str("source", source).
			Str("reason", result.Reason).
			Msg("JAILBREAK ATTEMPT DETECTED")
	}

	return &result
}
