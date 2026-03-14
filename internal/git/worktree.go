package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func UpdateMain(ctx context.Context, repoPath string) error {
	// Just fetch — this updates origin/main (or origin/master) without
	// touching the local branch, so it works even when main is checked out.
	if err := run(ctx, repoPath, "git", "fetch", "origin"); err != nil {
		return fmt.Errorf("fetching origin: %w", err)
	}
	return nil
}

// EnsureDetachedWorktree creates a temporary worktree checked out at the given
// ref (e.g. "origin/main"). Any stale worktree at worktreeDir is removed first.
func EnsureDetachedWorktree(ctx context.Context, repoPath, worktreeDir, ref string) error {
	RemoveWorktree(ctx, repoPath, worktreeDir)

	if err := run(ctx, repoPath, "git", "worktree", "add", "--detach", worktreeDir, ref); err != nil {
		return fmt.Errorf("creating detached worktree at %s: %w", ref, err)
	}
	return nil
}

// RemoveWorktree removes a worktree directory and prunes stale refs.
func RemoveWorktree(ctx context.Context, repoPath, worktreeDir string) {
	_ = run(ctx, repoPath, "git", "worktree", "remove", "--force", worktreeDir)
	_ = run(ctx, repoPath, "git", "worktree", "prune")
}

func EnsureWorktree(ctx context.Context, branch, repoPath, worktreeBase string) (string, error) {
	safeName := strings.ReplaceAll(branch, "/", "_")
	wtDir := filepath.Join(worktreeBase, safeName)

	// Check if worktree already exists
	out, err := output(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err == nil && strings.Contains(out, wtDir) {
		return wtDir, nil
	}

	// Check if branch exists locally
	if err := run(ctx, repoPath, "git", "rev-parse", "--verify", branch); err == nil {
		// Local branch exists, create worktree from it
		if err := run(ctx, repoPath, "git", "worktree", "add", wtDir, branch); err != nil {
			return "", fmt.Errorf("creating worktree from local branch: %w", err)
		}
		return wtDir, nil
	}

	// Check if branch exists on remote
	if err := run(ctx, repoPath, "git", "rev-parse", "--verify", "origin/"+branch); err == nil {
		// Remote branch exists
		if err := run(ctx, repoPath, "git", "worktree", "add", wtDir, "-b", branch, "origin/"+branch); err != nil {
			return "", fmt.Errorf("creating worktree from remote branch: %w", err)
		}
		return wtDir, nil
	}

	// New branch from default branch
	baseBranch := DefaultBranch(ctx, repoPath)
	if err := run(ctx, repoPath, "git", "worktree", "add", wtDir, "-b", branch, baseBranch); err != nil {
		return "", fmt.Errorf("creating worktree for new branch: %w", err)
	}
	return wtDir, nil
}

func CleanupWorktree(ctx context.Context, branch, repoPath, worktreeBase string) error {
	safeName := strings.ReplaceAll(branch, "/", "_")
	wtDir := filepath.Join(worktreeBase, safeName)

	RemoveWorktree(ctx, repoPath, wtDir)
	// Delete the local branch ref to prevent stale refs from interfering
	// with future worktree creation if the same issue is reopened.
	_ = run(ctx, repoPath, "git", "branch", "-D", branch)
	return nil
}

// EnsureWorktreeWithIdentity creates or reuses a worktree and configures the
// git author identity so commits are attributed to the bot.
func EnsureWorktreeWithIdentity(ctx context.Context, branch, repoPath, worktreeBase, userName, userEmail string) (string, error) {
	wtDir, err := EnsureWorktree(ctx, branch, repoPath, worktreeBase)
	if err != nil {
		return "", err
	}
	if userName != "" && userEmail != "" {
		if err := configureIdentity(ctx, wtDir, userName, userEmail); err != nil {
			return wtDir, fmt.Errorf("configuring git identity: %w", err)
		}
	}
	return wtDir, nil
}

func configureIdentity(ctx context.Context, dir, name, email string) error {
	if err := run(ctx, dir, "git", "config", "user.name", name); err != nil {
		return fmt.Errorf("setting git user.name: %w", err)
	}
	if err := run(ctx, dir, "git", "config", "user.email", email); err != nil {
		return fmt.Errorf("setting git user.email: %w", err)
	}
	return nil
}

func GetCurrentSHA(ctx context.Context, cwd string) (string, error) {
	out, err := output(ctx, cwd, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// DefaultBranch returns "main" or "master" depending on which exists in cwd.
func DefaultBranch(ctx context.Context, cwd string) string {
	if err := run(ctx, cwd, "git", "rev-parse", "--verify", "main"); err != nil {
		return "master"
	}
	return "main"
}

// DiffFromMain returns the diff between the default branch and HEAD.
func DiffFromMain(ctx context.Context, cwd string) (string, error) {
	base := DefaultBranch(ctx, cwd)
	return output(ctx, cwd, "git", "diff", base+"...HEAD")
}

// CommitLogFromMain returns the oneline commit log between the default branch and HEAD.
func CommitLogFromMain(ctx context.Context, cwd string) (string, error) {
	base := DefaultBranch(ctx, cwd)
	return output(ctx, cwd, "git", "log", base+"...HEAD", "--oneline")
}

// HasCommitsAheadOfMain returns true if HEAD has commits that the default branch does not.
func HasCommitsAheadOfMain(ctx context.Context, cwd string) (bool, error) {
	base := DefaultBranch(ctx, cwd)
	out, err := output(ctx, cwd, "git", "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "0", nil
}

// RevParse runs git rev-parse on the given ref and returns the resolved SHA.
func RevParse(ctx context.Context, cwd, ref string) (string, error) {
	out, err := output(ctx, cwd, "git", "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Output runs a git command and returns stdout. The first arg should be a git subcommand.
func Output(ctx context.Context, cwd string, args ...string) (string, error) {
	return output(ctx, cwd, "git", args...)
}

// Run executes a command in the given directory.
func Run(ctx context.Context, dir string, name string, args ...string) error {
	return run(ctx, dir, name, args...)
}

func run(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, string(out))
	}
	return nil
}

func output(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}
