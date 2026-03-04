from __future__ import annotations

import asyncio
import logging
import os
from pathlib import Path

from jinja2 import Environment, FileSystemLoader

log = logging.getLogger("codehephaestus.worker")

_PROMPTS_DIR = Path(__file__).resolve().parent.parent.parent / "prompts"

_TOOL_COMMANDS: dict[str, list[str]] = {
    "claude": ["claude", "--dangerously-skip-permissions", "--print"],
    "amp": ["amp", "--dangerously-allow-all"],
    "ccr": ["ccr", "code", "--dangerously-skip-permissions", "--print"],
}


def render_prompt(template_name: str, **context: object) -> str:
    """Render a Jinja2 prompt template."""
    env = Environment(
        loader=FileSystemLoader(str(_PROMPTS_DIR)),
        keep_trailing_newline=True,
    )
    return env.get_template(template_name).render(**context)


async def run_tool(
    *,
    prompt: str,
    working_dir: str,
    tool: str = "claude",
    dry_run: bool = False,
) -> tuple[int, str]:
    """Run AI tool once with the given prompt. Returns (exit_code, output)."""
    if dry_run:
        log.info("[DRY RUN] Would run %s in %s", tool, working_dir)
        return 0, ""

    cmd = _TOOL_COMMANDS.get(tool)
    if not cmd:
        raise ValueError(
            f"Unknown tool '{tool}'. Supported: {', '.join(_TOOL_COMMANDS)}"
        )

    env = {**os.environ, "DISABLE_PUSHOVER_NOTIFICATIONS": "true"}

    log.info("Running %s in %s...", tool, working_dir)

    proc = await asyncio.create_subprocess_exec(
        *cmd,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.STDOUT,
        cwd=working_dir,
        env=env,
    )

    stdout, _ = await proc.communicate(input=prompt.encode())
    output = stdout.decode()

    for line in output.splitlines():
        if line.strip():
            log.debug("worker: %s", line)

    exit_code = proc.returncode or 0
    log.info("Tool %s finished with exit code %d", tool, exit_code)
    return exit_code, output
