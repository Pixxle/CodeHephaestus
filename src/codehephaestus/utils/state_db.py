from __future__ import annotations

import logging
import sqlite3
from datetime import datetime, timezone
from pathlib import Path

log = logging.getLogger("codehephaestus.state_db")

CURRENT_SCHEMA_VERSION = 1


def _migrate_v1(conn: sqlite3.Connection) -> None:
    conn.executescript("""
        CREATE TABLE IF NOT EXISTS feedback_cutoffs (
            issue_key TEXT PRIMARY KEY,
            cutoff_utc TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS processed_shas (
            sha TEXT PRIMARY KEY,
            processed_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS attempt_records (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            issue_key TEXT NOT NULL,
            attempted_at TEXT NOT NULL
        );

        CREATE INDEX IF NOT EXISTS idx_attempts_issue
            ON attempt_records(issue_key);
    """)


_MIGRATIONS = [_migrate_v1]


class StateDB:
    def __init__(self, db_path: str | Path) -> None:
        db_path = Path(db_path)
        db_path.parent.mkdir(parents=True, exist_ok=True)

        self._conn = sqlite3.connect(str(db_path))
        self._conn.execute("PRAGMA journal_mode=WAL")
        self._conn.execute("PRAGMA synchronous=NORMAL")

        self._ensure_schema()
        log.info("StateDB opened: %s (version %d)", db_path, CURRENT_SCHEMA_VERSION)

    def _ensure_schema(self) -> None:
        self._conn.execute(
            "CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)"
        )
        row = self._conn.execute("SELECT version FROM schema_version").fetchone()
        current = row[0] if row else 0

        for i in range(current, CURRENT_SCHEMA_VERSION):
            log.info("Running migration v%d → v%d", i, i + 1)
            _MIGRATIONS[i](self._conn)

        if current == 0:
            self._conn.execute(
                "INSERT INTO schema_version (version) VALUES (?)",
                (CURRENT_SCHEMA_VERSION,),
            )
        elif current < CURRENT_SCHEMA_VERSION:
            self._conn.execute(
                "UPDATE schema_version SET version = ?", (CURRENT_SCHEMA_VERSION,)
            )
        self._conn.commit()

    # --- Feedback cutoffs ---

    def load_feedback_cutoffs(self) -> dict[str, datetime]:
        rows = self._conn.execute(
            "SELECT issue_key, cutoff_utc FROM feedback_cutoffs"
        ).fetchall()
        return {
            key: datetime.fromisoformat(ts).replace(tzinfo=timezone.utc)
            for key, ts in rows
        }

    def upsert_feedback_cutoff(self, issue_key: str, cutoff: datetime) -> None:
        self._conn.execute(
            "INSERT INTO feedback_cutoffs (issue_key, cutoff_utc) VALUES (?, ?)"
            " ON CONFLICT(issue_key) DO UPDATE SET cutoff_utc = excluded.cutoff_utc",
            (issue_key, cutoff.isoformat()),
        )
        self._conn.commit()

    # --- Processed SHAs ---

    def load_processed_shas(self) -> set[str]:
        rows = self._conn.execute("SELECT sha FROM processed_shas").fetchall()
        return {row[0] for row in rows}

    def insert_processed_sha(self, sha: str) -> None:
        self._conn.execute(
            "INSERT OR IGNORE INTO processed_shas (sha, processed_at) VALUES (?, ?)",
            (sha, datetime.now(timezone.utc).isoformat()),
        )
        self._conn.commit()

    def prune_old_shas(self, max_age_days: int = 90) -> int:
        cutoff = datetime.now(timezone.utc).isoformat()
        # Calculate cutoff by subtracting days
        from datetime import timedelta

        cutoff_dt = datetime.now(timezone.utc) - timedelta(days=max_age_days)
        cursor = self._conn.execute(
            "DELETE FROM processed_shas WHERE processed_at < ?",
            (cutoff_dt.isoformat(),),
        )
        self._conn.commit()
        return cursor.rowcount

    # --- Attempt records ---

    def load_attempt_records(self) -> dict[str, list[datetime]]:
        rows = self._conn.execute(
            "SELECT issue_key, attempted_at FROM attempt_records ORDER BY attempted_at"
        ).fetchall()
        result: dict[str, list[datetime]] = {}
        for key, ts in rows:
            dt = datetime.fromisoformat(ts).replace(tzinfo=timezone.utc)
            result.setdefault(key, []).append(dt)
        return result

    def insert_attempt(self, issue_key: str, attempted_at: datetime) -> None:
        self._conn.execute(
            "INSERT INTO attempt_records (issue_key, attempted_at) VALUES (?, ?)",
            (issue_key, attempted_at.isoformat()),
        )
        self._conn.commit()

    def prune_old_attempts(self, window_seconds: float) -> int:
        from datetime import timedelta

        cutoff_dt = datetime.now(timezone.utc) - timedelta(seconds=window_seconds)
        cursor = self._conn.execute(
            "DELETE FROM attempt_records WHERE attempted_at < ?",
            (cutoff_dt.isoformat(),),
        )
        self._conn.commit()
        return cursor.rowcount

    def close(self) -> None:
        self._conn.close()
