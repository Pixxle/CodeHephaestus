package changelog

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/pixxle/solomon/internal/db"
	"github.com/pixxle/solomon/internal/git"
	ghclient "github.com/pixxle/solomon/internal/github"
)

// Changelog holds the structured result of a commit collection.
type Changelog struct {
	RepoName   string
	Commits    []CommitEntry
	IsFirstRun bool
	FromSHA    string
	ToSHA      string
}

// CommitEntry represents a single parsed commit, optionally enriched with PR metadata.
type CommitEntry struct {
	SHA     string
	Subject string
	Author  string
	Date    string
	// PR metadata (populated when a GitHub client is available)
	PRNumber int
	PRTitle  string
	PRBody   string
}

func collectCommits(ctx context.Context, repoPath, repoName, instanceID string, maxCommits int, stateDB *db.StateDB) (*Changelog, error) {
	if err := git.UpdateMain(ctx, repoPath); err != nil {
		return nil, fmt.Errorf("fetching origin: %w", err)
	}

	defaultBranch := git.DefaultBranch(ctx, repoPath)
	originRef := "origin/" + defaultBranch

	headSHA, err := git.RevParse(ctx, repoPath, originRef)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", originRef, err)
	}

	lastRun, err := stateDB.GetChangelogRun(instanceID, repoName)
	if err != nil {
		return nil, fmt.Errorf("getting last changelog run: %w", err)
	}

	cl := &Changelog{
		RepoName: repoName,
		ToSHA:    headSHA,
	}

	var logArgs []string
	if lastRun == nil {
		cl.IsFirstRun = true
		logArgs = []string{"log", originRef, fmt.Sprintf("-%d", maxCommits), "--format=%H|%s|%an|%aI"}
	} else {
		cl.FromSHA = lastRun.LastCommitSHA
		if cl.FromSHA == headSHA {
			return cl, nil
		}
		logArgs = []string{"log", cl.FromSHA + ".." + originRef, "--format=%H|%s|%an|%aI"}
	}

	out, err := git.Output(ctx, repoPath, logArgs...)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		cl.Commits = append(cl.Commits, CommitEntry{
			SHA:     parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}

	return cl, nil
}

// enrichWithPRData looks up PR metadata for each commit via the GitHub API.
func enrichWithPRData(ctx context.Context, cl *Changelog, ghClient *ghclient.Client) {
	for i := range cl.Commits {
		pr, err := ghClient.GetPRForCommit(ctx, cl.Commits[i].SHA)
		if err != nil {
			log.Debug().Err(err).Str("sha", cl.Commits[i].SHA).Msg("changelog: could not look up PR for commit")
			continue
		}
		if pr != nil {
			cl.Commits[i].PRNumber = pr.Number
			cl.Commits[i].PRTitle = pr.Title
			cl.Commits[i].PRBody = pr.Body
		}
	}
}
