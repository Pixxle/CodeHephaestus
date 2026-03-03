from __future__ import annotations

import logging
import re
from datetime import datetime, timezone

import httpx

from codehephaestus.config import Settings
from codehephaestus.trackers.base import TaskTracker
from codehephaestus.trackers.models import Comment, Issue

log = logging.getLogger("codehephaestus.jira")


def _slugify(text: str, max_len: int = 40) -> str:
    slug = re.sub(r"[^a-zA-Z0-9]+", "-", text).strip("-").lower()
    return slug[:max_len].rstrip("-")


def _parse_datetime(value: str | None) -> datetime | None:
    if not value:
        return None
    # Jira uses ISO 8601 with timezone offset like 2024-01-15T10:30:00.000+0000
    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None


def _extract_text(adf: dict | None) -> str:
    """Extract plain text from Jira's Atlassian Document Format."""
    if not adf or not isinstance(adf, dict):
        return ""
    parts: list[str] = []
    for node in adf.get("content", []):
        if node.get("type") == "paragraph":
            for inline in node.get("content", []):
                if inline.get("type") == "text":
                    parts.append(inline.get("text", ""))
            parts.append("\n")
        elif node.get("type") == "text":
            parts.append(node.get("text", ""))
    return "".join(parts).strip()


class JiraTracker(TaskTracker):
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._base = settings.tracker_base_url.rstrip("/")
        self._project = settings.tracker_project
        self._label = settings.tracker_label
        self._client = httpx.AsyncClient(
            base_url=f"{self._base}/rest/api/3",
            auth=(settings.tracker_email, settings.tracker_api_key),
            headers={"Accept": "application/json"},
            timeout=30.0,
        )
        self._status_map = {
            "todo": settings.jira_status_todo,
            "in_progress": settings.jira_status_in_progress,
            "in_review": settings.jira_status_in_review,
            "done": settings.jira_status_done,
        }

    async def validate_connection(self) -> bool:
        resp = await self._client.get("/myself")
        resp.raise_for_status()
        user = resp.json()
        log.info(
            "Connected as %s | Project: %s | Label: %s",
            user.get("emailAddress", user.get("displayName", "unknown")),
            self._project,
            self._label,
        )
        return True

    async def fetch_issues_by_status(self, status: str) -> list[Issue]:
        jql = (
            f'project = "{self._project}" '
            f'AND labels = "{self._label}" '
            f'AND status = "{status}"'
        )
        log.debug("JQL: %s", jql)
        resp = await self._client.post(
            "/search/jql",
            json={
                "jql": jql,
                "fields": ["summary", "description", "status", "labels", "created", "updated"],
                "maxResults": 50,
            },
        )
        resp.raise_for_status()
        data = resp.json()

        issues: list[Issue] = []
        for item in data.get("issues", []):
            fields = item["fields"]
            issues.append(
                Issue(
                    key=item["key"],
                    title=fields.get("summary", ""),
                    description=_extract_text(fields.get("description")),
                    status=fields.get("status", {}).get("name", ""),
                    labels=fields.get("labels", []),
                    created=_parse_datetime(fields.get("created")),
                    updated=_parse_datetime(fields.get("updated")),
                )
            )
        return issues

    async def transition_issue(self, issue_key: str, to_status: str) -> None:
        resp = await self._client.get(f"/issue/{issue_key}/transitions")
        resp.raise_for_status()
        transitions = resp.json().get("transitions", [])

        target = next(
            (t for t in transitions if t["name"].lower() == to_status.lower()),
            None,
        )
        if not target:
            available = [t["name"] for t in transitions]
            log.warning(
                "%s: transition to '%s' not found. Available: %s",
                issue_key,
                to_status,
                available,
            )
            return

        resp = await self._client.post(
            f"/issue/{issue_key}/transitions",
            json={"transition": {"id": target["id"]}},
        )
        resp.raise_for_status()
        log.info("%s: transitioned → %s", issue_key, to_status)

    def get_issue_branch_name(self, issue: Issue) -> str:
        return f"ralph/{issue.key}-{_slugify(issue.title)}"

    async def get_comments(self, issue_key: str) -> list[Comment]:
        resp = await self._client.get(f"/issue/{issue_key}/comment")
        resp.raise_for_status()
        data = resp.json()

        comments: list[Comment] = []
        for c in data.get("comments", []):
            comments.append(
                Comment(
                    id=c["id"],
                    author=c.get("author", {}).get("displayName", "unknown"),
                    body=_extract_text(c.get("body")),
                    created=_parse_datetime(c.get("created")),
                )
            )
        return comments

    async def add_comment(self, issue_key: str, body: str) -> None:
        resp = await self._client.post(
            f"/issue/{issue_key}/comment",
            json={
                "body": {
                    "type": "doc",
                    "version": 1,
                    "content": [
                        {
                            "type": "paragraph",
                            "content": [{"type": "text", "text": body}],
                        }
                    ],
                }
            },
        )
        resp.raise_for_status()
        log.debug("%s: comment added", issue_key)

    async def close(self) -> None:
        await self._client.aclose()
