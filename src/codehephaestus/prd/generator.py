from __future__ import annotations

import asyncio
import json
import logging
import re
from pathlib import Path

from jinja2 import Environment, FileSystemLoader

from codehephaestus.prd.models import PRDContext

log = logging.getLogger("codehephaestus.prd")

_PROMPTS_DIR = Path(__file__).resolve().parent.parent.parent.parent / "prompts"

# Same tool commands as ralph runner — PRD generation uses the same AI tool
_TOOL_COMMANDS: dict[str, list[str]] = {
    "claude": ["claude", "--dangerously-skip-permissions", "--print"],
    "amp": ["amp", "--dangerously-allow-all"],
    "ccr": ["ccr", "code", "--dangerously-skip-permissions", "--print"],
}


def _render_prompt(template_name: str, **context: object) -> str:
    env = Environment(
        loader=FileSystemLoader(str(_PROMPTS_DIR)),
        keep_trailing_newline=True,
    )
    return env.get_template(template_name).render(**context)


def _slugify(text: str, max_len: int = 40) -> str:
    return re.sub(r"[^a-zA-Z0-9]+", "-", text).strip("-").lower()[:max_len].rstrip("-")


async def _call_tool(
    tool: str, prompt: str, working_dir: str
) -> tuple[int, str, str]:
    """Call an AI tool with a prompt via stdin. Returns (exit_code, stdout, stderr)."""
    cmd = _TOOL_COMMANDS.get(tool)
    if not cmd:
        raise ValueError(f"Unknown tool '{tool}'. Supported: {', '.join(_TOOL_COMMANDS)}")

    proc = await asyncio.create_subprocess_exec(
        *cmd,
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
        cwd=working_dir,
    )
    stdout, stderr = await proc.communicate(input=prompt.encode())
    return proc.returncode or 0, stdout.decode(), stderr.decode()


def _extract_json(text: str) -> str:
    """Extract JSON from AI output, stripping markdown fences if present."""
    # Try to find JSON in code fences first
    match = re.search(r"```(?:json)?\s*\n(.*?)```", text, re.DOTALL)
    if match:
        return match.group(1).strip()
    # Otherwise try to find raw JSON object
    match = re.search(r"\{.*\}", text, re.DOTALL)
    if match:
        return match.group(0).strip()
    return text.strip()


async def generate_prd(
    ctx: PRDContext,
    *,
    working_dir: str,
    branch_name: str,
    tool: str = "claude",
    dry_run: bool = False,
) -> Path:
    """Generate a PRD and convert it to prd.json for ralph.

    Two-step process:
    1. Call AI tool with PRD skill prompt → structured markdown PRD
    2. Call AI tool with ralph converter prompt → prd.json

    Returns the path to the generated prd.json file.
    """
    ralph_dir = Path(working_dir) / "scripts" / "ralph"
    ralph_dir.mkdir(parents=True, exist_ok=True)
    prd_json_path = ralph_dir / "prd.json"

    # Also save the markdown PRD for reference
    tasks_dir = Path(working_dir) / "tasks"
    tasks_dir.mkdir(parents=True, exist_ok=True)
    prd_md_path = tasks_dir / f"prd-{_slugify(ctx.issue_key)}.md"

    if dry_run:
        log.info("[DRY RUN] Would generate PRD for %s at %s", ctx.issue_key, prd_json_path)
        return prd_json_path

    # Step 1: Generate structured markdown PRD
    log.info("Step 1: Generating markdown PRD for %s using %s...", ctx.issue_key, tool)
    prd_prompt = _render_prompt(
        "prd_generate.md.j2",
        issue_key=ctx.issue_key,
        title=ctx.title,
        description=ctx.description,
        comments=ctx.comments,
        check_output=ctx.check_output,
    )

    rc, prd_markdown, err = await _call_tool(tool, prd_prompt, working_dir)
    if rc != 0:
        log.error("PRD generation failed (exit=%d): %s", rc, err.strip())
        prd_markdown = (
            f"# {ctx.issue_key}: {ctx.title}\n\n"
            f"## Overview\n\n{ctx.description}\n"
        )

    prd_md_path.write_text(prd_markdown.strip() + "\n")
    log.info("Saved markdown PRD at %s (%d bytes)", prd_md_path, len(prd_markdown))

    # Step 2: Convert markdown PRD → prd.json
    log.info("Step 2: Converting PRD to prd.json for ralph...")
    convert_prompt = _render_prompt(
        "prd_to_json.md.j2",
        prd_markdown=prd_markdown,
        branch_name=branch_name,
        project_name=ctx.issue_key.split("-")[0] if "-" in ctx.issue_key else "Project",
    )

    rc, json_output, err = await _call_tool(tool, convert_prompt, working_dir)
    if rc != 0:
        log.error("PRD→JSON conversion failed (exit=%d): %s", rc, err.strip())
        _write_fallback_prd_json(prd_json_path, ctx, branch_name)
        return prd_json_path

    # Parse and validate the JSON
    raw_json = _extract_json(json_output)
    try:
        prd_data = json.loads(raw_json)
    except json.JSONDecodeError as exc:
        log.error("Failed to parse prd.json from AI output: %s", exc)
        log.debug("Raw output:\n%s", json_output)
        _write_fallback_prd_json(prd_json_path, ctx, branch_name)
        return prd_json_path

    # Ensure required fields
    prd_data.setdefault("project", ctx.issue_key.split("-")[0])
    prd_data.setdefault("branchName", branch_name)
    prd_data.setdefault("description", f"{ctx.issue_key}: {ctx.title}")
    prd_data.setdefault("userStories", [])

    # Ensure all stories have required fields
    for story in prd_data["userStories"]:
        story.setdefault("passes", False)
        story.setdefault("notes", "")
        story.setdefault("priority", 1)

    prd_json_path.write_text(json.dumps(prd_data, indent=2) + "\n")
    log.info(
        "Generated prd.json at %s (%d user stories)",
        prd_json_path,
        len(prd_data["userStories"]),
    )
    return prd_json_path


def _write_fallback_prd_json(
    path: Path, ctx: PRDContext, branch_name: str
) -> None:
    """Write a minimal prd.json as fallback when AI conversion fails."""
    fallback = {
        "project": ctx.issue_key.split("-")[0] if "-" in ctx.issue_key else "Project",
        "branchName": branch_name,
        "description": f"{ctx.issue_key}: {ctx.title}",
        "userStories": [
            {
                "id": "US-001",
                "title": ctx.title,
                "description": ctx.description or f"Implement {ctx.title}",
                "acceptanceCriteria": [
                    "Feature works as described",
                    "Typecheck passes",
                ],
                "priority": 1,
                "passes": False,
                "notes": "",
            }
        ],
    }
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(fallback, indent=2) + "\n")
    log.warning("Wrote fallback prd.json at %s", path)
