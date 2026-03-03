from __future__ import annotations

from codehephaestus.config import Settings, TaskTrackerType
from codehephaestus.trackers.base import TaskTracker
from codehephaestus.trackers.jira import JiraTracker


def create_tracker(settings: Settings) -> TaskTracker:
    if settings.task_tracker == TaskTrackerType.JIRA:
        return JiraTracker(settings)
    raise ValueError(f"Unsupported tracker: {settings.task_tracker}")


__all__ = ["TaskTracker", "create_tracker"]
