from __future__ import annotations

import asyncio
import logging
import shutil
from datetime import datetime, timezone
from pathlib import Path

from jinja2 import Environment, FileSystemLoader

log = logging.getLogger("codehephaestus.ralph")

_COMPLETION_SIGNAL = "<promise>COMPLETE</promise>"
_PROMPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "prompts"

# Tool commands: how to invoke each supported AI coding tool
_TOOL_COMMANDS: dict[str, list[str]] = {
    "claude": ["claude", "--dangerously-skip-permissions", "--print"],
    "amp": ["amp", "--dangerously-allow-all"],
    "ccr": ["ccr", "code", "--dangerously-skip-permissions", "--print"],
}


def _render_ralph_prompt(prd_path: Path, progress_path: Path) -> str:
    """Render the ralph iteration prompt with PRD and progress context."""
    env = Environment(
        loader=FileSystemLoader(str(_PROMPTS_DIR)),
        keep_trailing_newline=True,
    )
    tmpl = env.get_template("ralph_iteration.md.j2")

    prd_content = prd_path.read_text() if prd_path.exists() else ""
    progress_content = progress_path.read_text() if progress_path.exists() else ""

    return tmpl.render(prd=prd_content, progress=progress_content)


def _archive_previous_run(ralph_dir: Path) -> None:
    """Archive previous progress file if it exists."""
    progress_file = ralph_dir / "progress.txt"

    if not progress_file.exists():
        return

    stamp = datetime.now(timezone.utc).strftime("%Y-%m-%d-%H%M%S")
    archive_dir = ralph_dir / "archive" / stamp
    archive_dir.mkdir(parents=True, exist_ok=True)

    # Archive any PRD files present
    for prd_file in ralph_dir.glob("prd*"):
        if prd_file.is_file():
            shutil.copy2(prd_file, archive_dir / prd_file.name)
    shutil.copy2(progress_file, archive_dir / "progress.txt")
    log.debug("Archived previous run to %s", archive_dir)


def _init_progress(ralph_dir: Path) -> Path:
    """Create a fresh progress file. Returns its path."""
    ralph_dir.mkdir(parents=True, exist_ok=True)
    progress = ralph_dir / "progress.txt"
    progress.write_text(
        f"# Ralph Progress Log\nStarted: {datetime.now(timezone.utc).isoformat()}\n---\n"
    )
    return progress


async def _run_tool_iteration(
    tool: str,
    prompt: str,
    working_dir: str,
) -> tuple[int, str]:
    """Run one iteration of the AI tool. Returns (exit_code, output)."""
    cmd = _TOOL_COMMANDS.get(tool)
    if not cmd:
        raise ValueError(
            f"Unknown tool '{tool}'. Supported: {', '.join(_TOOL_COMMANDS)}"
        )

    proc = await asyncio.create_subprocess_exec(
        *cmd,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        cwd=working_dir,
        env={
            **__import__("os").environ,
            "DISABLE_PUSHOVER_NOTIFICATIONS": "true",
            "RALPH_LOOP": "true",
        },
    )

    stdout, _ = await proc.communicate(input=prompt.encode())
    output = stdout.decode()

    # Stream output to debug log line by line
    for line in output.splitlines():
        if line.strip():
            log.debug("ralph: %s", line)

    return proc.returncode or 0, output


async def run_ralph(
    *,
    prd_path: Path,
    working_dir: str,
    tool: str = "claude",
    max_iterations: int = 10,
    dry_run: bool = False,
) -> int:
    """Run the ralph loop: iterate an AI coding tool until completion or max iterations.

    Returns 0 on successful completion, 1 if max iterations reached without completion.
    """
    if dry_run:
        log.info("[DRY RUN] Would run ralph loop with PRD %s in %s", prd_path, working_dir)
        return 0

    ralph_dir = Path(working_dir) / "scripts" / "ralph"
    _archive_previous_run(ralph_dir)
    progress_path = _init_progress(ralph_dir)

    log.info("Starting ralph loop (tool=%s, max %d iterations)...", tool, max_iterations)

    for i in range(1, max_iterations + 1):
        log.info("=== Ralph iteration %d of %d ===", i, max_iterations)

        prompt = _render_ralph_prompt(prd_path, progress_path)
        exit_code, output = await _run_tool_iteration(tool, prompt, working_dir)

        # Append iteration result to progress
        with progress_path.open("a") as f:
            f.write(f"\n## Iteration {i}\n")
            f.write(f"Exit code: {exit_code}\n")
            # Store a summary (last 20 lines) to keep file manageable
            lines = output.strip().splitlines()
            summary = "\n".join(lines[-20:]) if len(lines) > 20 else output.strip()
            f.write(f"```\n{summary}\n```\n")

        if _COMPLETION_SIGNAL in output:
            log.info("Ralph completed all tasks at iteration %d of %d", i, max_iterations)
            return 0

        if exit_code != 0:
            log.warning("Ralph iteration %d exited with code %d", i, exit_code)

        if i < max_iterations:
            await asyncio.sleep(2)

    log.warning("Ralph reached max iterations (%d) without completing", max_iterations)
    return 1
