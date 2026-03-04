from __future__ import annotations

import logging
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path

log = logging.getLogger("codehephaestus.loop_prevention")

MAX_ATTEMPTS = 5
WINDOW_SECONDS = 600  # 10 minutes


@dataclass
class AttemptRecord:
    timestamps: list[datetime] = field(default_factory=list)


class LoopPrevention:
    def __init__(
        self,
        max_attempts: int = MAX_ATTEMPTS,
        window_seconds: float = WINDOW_SECONDS,
        db_path: str | Path | None = None,
    ) -> None:
        self._max_attempts = max_attempts
        self._window = window_seconds
        self._attempts: dict[str, AttemptRecord] = {}
        self._processed_shas: set[str] = set()
        self._feedback_cutoffs: dict[str, datetime] = {}
        self._db = None

        if db_path is not None:
            from codehephaestus.utils.state_db import StateDB

            self._db = StateDB(db_path)
            self._feedback_cutoffs = self._db.load_feedback_cutoffs()
            self._processed_shas = self._db.load_processed_shas()
            for key, timestamps in self._db.load_attempt_records().items():
                self._attempts[key] = AttemptRecord(timestamps=timestamps)
            log.info(
                "Loaded persisted state: %d cutoffs, %d SHAs, %d attempt keys",
                len(self._feedback_cutoffs),
                len(self._processed_shas),
                len(self._attempts),
            )

    def should_skip(self, issue_key: str) -> bool:
        record = self._attempts.get(issue_key)
        if not record:
            return False
        now = datetime.now(timezone.utc)
        cutoff = now - timedelta(seconds=self._window)
        recent = [t for t in record.timestamps if t > cutoff]
        record.timestamps = recent
        if len(recent) >= self._max_attempts:
            log.warning(
                "%s: skipping — %d attempts in last %ds",
                issue_key,
                len(recent),
                int(self._window),
            )
            return True
        return False

    def record_attempt(self, issue_key: str) -> None:
        if issue_key not in self._attempts:
            self._attempts[issue_key] = AttemptRecord()
        now = datetime.now(timezone.utc)
        self._attempts[issue_key].timestamps.append(now)
        if self._db:
            self._db.insert_attempt(issue_key, now)

    def is_sha_processed(self, sha: str) -> bool:
        return sha in self._processed_shas

    def mark_sha_processed(self, sha: str) -> None:
        self._processed_shas.add(sha)
        if self._db:
            self._db.insert_processed_sha(sha)

    def get_feedback_cutoff(self, issue_key: str) -> datetime | None:
        """Get the cutoff timestamp — only comments after this are 'new'."""
        return self._feedback_cutoffs.get(issue_key)

    def mark_feedback_processed(self, issue_key: str) -> None:
        """Record that we've addressed all feedback up to now."""
        now = datetime.now(timezone.utc)
        self._feedback_cutoffs[issue_key] = now
        if self._db:
            self._db.upsert_feedback_cutoff(issue_key, now)

    def close(self) -> None:
        if self._db:
            self._db.close()
            self._db = None
