from __future__ import annotations

from dataclasses import dataclass


@dataclass
class PRDContext:
    """Context passed to the PRD generation AI call."""

    issue_key: str
    title: str
    description: str
    comments: list[dict[str, str]] | None = None
    check_output: str | None = None
