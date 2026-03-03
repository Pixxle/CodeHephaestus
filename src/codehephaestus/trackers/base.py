from __future__ import annotations

from abc import ABC, abstractmethod

from codehephaestus.trackers.models import Comment, Issue


class TaskTracker(ABC):
    @abstractmethod
    async def validate_connection(self) -> bool: ...

    @abstractmethod
    async def fetch_issues_by_status(self, status: str) -> list[Issue]: ...

    @abstractmethod
    async def transition_issue(self, issue_key: str, to_status: str) -> None: ...

    @abstractmethod
    def get_issue_branch_name(self, issue: Issue) -> str: ...

    @abstractmethod
    async def get_comments(self, issue_key: str) -> list[Comment]: ...

    @abstractmethod
    async def add_comment(self, issue_key: str, body: str) -> None: ...
