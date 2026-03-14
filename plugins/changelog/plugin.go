package changelog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/pixxle/solomon/internal/plugin"
)

func init() {
	plugin.Register("changelog", func(cfg plugin.PluginConfig, libs *plugin.SharedLibs) (plugin.Plugin, error) {
		return NewChangelogPlugin(cfg, libs)
	})
}

// ChangelogPlugin generates changelogs from git history and outputs them to Slack and/or PRs.
type ChangelogPlugin struct {
	pluginCfg  plugin.PluginConfig
	libs       *plugin.SharedLibs
	instanceID string
	output     []string
	maxCommits int
	cron       *cron.Cron
	mu         sync.Mutex
	cancel     context.CancelFunc
}

func NewChangelogPlugin(cfg plugin.PluginConfig, libs *plugin.SharedLibs) (*ChangelogPlugin, error) {
	return &ChangelogPlugin{
		pluginCfg:  cfg,
		libs:       libs,
		instanceID: plugin.SettingString(cfg.Settings, "instance_id", "changelog-0"),
		output:     plugin.SettingStringSlice(cfg.Settings, "output", []string{"slack"}),
		maxCommits: plugin.SettingInt(cfg.Settings, "max_commits", 100),
	}, nil
}

func (p *ChangelogPlugin) Name() string { return "changelog" }

func (p *ChangelogPlugin) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	if p.libs.Config.Once {
		defer p.cancel()
		log.Info().Msg("changelog: running (once mode)")
		if err := p.run(ctx); err != nil {
			log.Error().Err(err).Msg("changelog run failed")
		}
		return nil
	}

	p.cron = cron.New()
	for _, sched := range p.pluginCfg.Schedules {
		s := sched
		_, err := p.cron.AddFunc(s.CronExpr, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if ctx.Err() != nil {
				return
			}
			if err := p.run(ctx); err != nil {
				log.Error().Err(err).Str("trigger", s.Name).Msg("changelog run failed")
			}
		})
		if err != nil {
			return fmt.Errorf("add cron schedule %q: %w", s.Name, err)
		}
		log.Info().Str("schedule", s.Name).Str("cron", s.CronExpr).Msg("changelog: registered cron schedule")
	}

	p.cron.Start()
	return nil
}

func (p *ChangelogPlugin) Stop(ctx context.Context) error {
	if p.cron != nil {
		p.cron.Stop()
	}
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func (p *ChangelogPlugin) run(ctx context.Context) error {
	if len(p.pluginCfg.Repos) == 0 {
		return fmt.Errorf("changelog plugin requires at least one repo")
	}

	repo := p.pluginCfg.Repos[0]
	repoPath := repo.Path
	if repoPath == "" || repoPath == "." {
		repoPath = p.libs.Config.TargetRepoPath
	}

	changelog, err := collectCommits(ctx, repoPath, repo.Name, p.instanceID, p.maxCommits, p.libs.DB)
	if err != nil {
		return fmt.Errorf("collecting commits: %w", err)
	}

	if len(changelog.Commits) == 0 && !changelog.IsFirstRun {
		log.Info().Str("repo", repo.Name).Msg("changelog: no new commits")
		return nil
	}

	// Enrich commits with PR metadata when a GitHub client is available
	if ghClient := p.libs.GitHub[repo.Name]; ghClient != nil {
		enrichWithPRData(ctx, changelog, ghClient)
	}

	now := time.Now()
	model := p.libs.Config.PlanningModel
	tmplData := buildTemplateData(changelog, now)

	for _, method := range p.output {
		switch method {
		case "slack":
			channelID := plugin.SettingString(p.pluginCfg.Settings, "slack_channel_id", p.libs.Config.SlackChannelID)
			if p.libs.SlackClient == nil {
				log.Warn().Msg("changelog: slack output requested but no slack client configured")
				continue
			}
			if err := outputSlack(ctx, changelog, p.libs.SlackClient, channelID, repoPath, model, tmplData); err != nil {
				return fmt.Errorf("slack output: %w", err)
			}
		case "pr":
			ghClient := p.libs.GitHub[repo.Name]
			if ghClient == nil {
				log.Warn().Str("repo", repo.Name).Msg("changelog: pr output requested but no github client for repo")
				continue
			}
			changelogPath := plugin.SettingString(p.pluginCfg.Settings, "changelog_path", "CHANGELOG.md")
			if err := outputPR(ctx, changelog, repoPath, ghClient, p.libs.Config, changelogPath, model, tmplData, now); err != nil {
				return fmt.Errorf("pr output: %w", err)
			}
		default:
			log.Warn().Str("method", method).Msg("changelog: unknown output method")
		}
	}

	if err := p.libs.DB.UpsertChangelogRun(p.instanceID, repo.Name, changelog.ToSHA); err != nil {
		return fmt.Errorf("upserting changelog run: %w", err)
	}

	log.Info().
		Str("repo", repo.Name).
		Int("commits", len(changelog.Commits)).
		Bool("first_run", changelog.IsFirstRun).
		Msg("changelog: run complete")
	return nil
}
