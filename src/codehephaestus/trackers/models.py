from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime


@dataclass
class Issue:
    key: str
    title: str
    description: str = ""
    status: str = ""
    labels: list[str] = field(default_factory=list)
    created: datetime | None = None
    updated: datetime | None = None


@dataclass
class Comment:
    id: str
    author: str
    body: str
    created: datetime | None = None


@dataclass
class Transition:
    id: str
    name: str
