package slack

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
	slackapi "github.com/slack-go/slack"

	"github.com/pixxle/solomon/internal/claude"
	"github.com/pixxle/solomon/internal/config"
	"github.com/pixxle/solomon/internal/db"
)

// standupItem is the JSON shape passed to the standup prompt template.
type standupItem struct {
	IssueKey    string `json:"issue_key"`
	Status      string `json:"status"`
	Phase       string `json:"phase"`
	Questions   string `json:"questions"`
	Updated     string `json:"updated"`
	WaitingDays int    `json:"waiting_days,omitempty"`
}

// StandupRunner posts a daily LLM-generated standup message.
type StandupRunner struct {
	client       *slackapi.Client
	channelID    string
	standupHour  int
	stateDB      *db.StateDB
	cfg          *config.Config
	lastPostedAt time.Time // persisted timestamp of the last standup for delta queries and dedup
}

// NewStandupRunner creates a StandupRunner that posts to the given channel at the given hour.
// Accepts a shared Slack client to avoid duplicate HTTP connection pools.
func NewStandupRunner(cfg *config.Config, stateDB *db.StateDB, client *slackapi.Client, standupHour int, channelID string) *StandupRunner {
	s := &StandupRunner{
		client:      client,
		channelID:   channelID,
		standupHour: standupHour,
		stateDB:     stateDB,
		cfg:         cfg,
	}
	// Restore last standup time from DB so delta works across restarts.
	if t, err := stateDB.GetLastStandupTime(); err == nil && !t.IsZero() {
		s.lastPostedAt = t
	}
	return s
}

// Run starts the standup ticker. Blocks until ctx is cancelled.
func (s *StandupRunner) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.check(ctx)
		}
	}
}

func (s *StandupRunner) check(ctx context.Context) {
	now := time.Now().UTC()

	if now.Hour() != s.standupHour {
		return
	}
	if !s.lastPostedAt.IsZero() && s.lastPostedAt.Format("2006-01-02") == now.Format("2006-01-02") {
		return
	}

	if err := s.postStandup(ctx, now); err != nil {
		log.Error().Err(err).Msg("failed to post standup")
	}
}

func (s *StandupRunner) postStandup(ctx context.Context, now time.Time) error {
	// Always fetch active states — needed for blocked-ticket merge.
	active, err := s.stateDB.GetActivePlanningStates()
	if err != nil {
		return err
	}

	// Determine the changed set.
	var changed []*db.PlanningState
	if s.lastPostedAt.IsZero() {
		changed = active
	} else {
		changed, err = s.stateDB.GetPlanningStatesUpdatedSince(s.lastPostedAt)
		if err != nil {
			return err
		}
	}

	// Merge blocked tickets (active with open questions) not already in the changed set.
	seen := make(map[string]bool, len(changed))
	for _, ps := range changed {
		seen[ps.IssueKey] = true
	}
	for _, ps := range active {
		if seen[ps.IssueKey] {
			continue
		}
		if ps.HasOpenQuestions() {
			changed = append(changed, ps)
		}
	}

	if len(changed) == 0 {
		log.Info().Msg("standup: no ticket changes or blocked items to report, skipping")
		s.lastPostedAt = now
		return s.stateDB.SetLastStandupTime(now)
	}

	items := make([]standupItem, len(changed))
	for i, ps := range changed {
		item := standupItem{
			IssueKey:  ps.IssueKey,
			Status:    ps.Status,
			Phase:     ps.PlanningPhase,
			Questions: ps.QuestionsJSON,
			Updated:   ps.UpdatedAt.Format(time.RFC3339),
		}
		if ps.LastSystemCommentAt != nil && ps.HasOpenQuestions() {
			item.WaitingDays = int(now.Sub(*ps.LastSystemCommentAt).Hours() / 24)
		}
		items[i] = item
	}
	itemsJSON, _ := json.Marshal(items)

	sinceLabel := "yesterday"
	if !s.lastPostedAt.IsZero() {
		sinceLabel = s.lastPostedAt.Format("2006-01-02 15:04 UTC")
	}

	prompt, err := claude.RenderPrompt("standup.md.tmpl", map[string]interface{}{
		"Date":           now.Format("2006-01-02"),
		"BotDisplayName": s.cfg.BotDisplayName,
		"TicketData":     string(itemsJSON),
		"Since":          sinceLabel,
	})
	if err != nil {
		return err
	}

	result, err := claude.RunClaudeText(ctx, prompt, s.cfg.TargetRepoPath, s.cfg.PlanningModel)
	if err != nil {
		return err
	}

	_, _, err = s.client.PostMessageContext(ctx, s.channelID,
		slackapi.MsgOptionText(result.Output, false),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to post standup to Slack")
		return err
	}

	s.lastPostedAt = now
	if err := s.stateDB.SetLastStandupTime(now); err != nil {
		log.Warn().Err(err).Msg("failed to persist last standup time")
	}

	log.Info().Str("channel", s.channelID).Msg("standup posted")
	return nil
}
