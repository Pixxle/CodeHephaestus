from __future__ import annotations

import asyncio
import logging
from pathlib import Path

log = logging.getLogger("codehephaestus.git")


async def _run(cmd: list[str], cwd: str | Path | None = None) -> str:
    proc = await asyncio.create_subprocess_exec(
        *cmd,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        cwd=cwd,
    )
    stdout, stderr = await proc.communicate()
    if proc.returncode != 0:
        raise RuntimeError(
            f"Command {cmd!r} failed (rc={proc.returncode}): {stderr.decode().strip()}"
        )
    return stdout.decode().strip()


async def ensure_branch(branch: str, repo_path: str) -> str:
    """Create branch if it doesn't exist and check it out. Returns the repo path."""
    try:
        await _run(["git", "checkout", branch], cwd=repo_path)
        log.info("Checked out existing branch %s", branch)
    except RuntimeError:
        await _run(["git", "checkout", "-b", branch], cwd=repo_path)
        log.info("Created and checked out new branch %s", branch)
    return repo_path


async def worktree_add(branch: str, repo_path: str) -> str:
    """Create a git worktree for the branch. Returns the worktree path."""
    wt_dir = Path(repo_path) / ".worktrees" / branch.replace("/", "_")
    if wt_dir.exists():
        log.info("Worktree already exists at %s", wt_dir)
        return str(wt_dir)

    try:
        # Try adding worktree for existing branch
        await _run(
            ["git", "worktree", "add", str(wt_dir), branch],
            cwd=repo_path,
        )
    except RuntimeError:
        # Branch doesn't exist yet — create it
        await _run(
            ["git", "worktree", "add", "-b", branch, str(wt_dir)],
            cwd=repo_path,
        )
    log.info("Created worktree at %s for branch %s", wt_dir, branch)
    return str(wt_dir)


async def worktree_remove(branch: str, repo_path: str) -> None:
    wt_dir = Path(repo_path) / ".worktrees" / branch.replace("/", "_")
    if wt_dir.exists():
        await _run(["git", "worktree", "remove", str(wt_dir), "--force"], cwd=repo_path)
        log.info("Removed worktree at %s", wt_dir)


async def get_current_sha(cwd: str) -> str:
    return await _run(["git", "rev-parse", "HEAD"], cwd=cwd)
