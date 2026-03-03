from __future__ import annotations

import argparse
import asyncio
import sys

from codehephaestus.config import Settings
from codehephaestus.logging_config import setup_logging
from codehephaestus.loop import run_loop


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="codehephaestus",
        description="Autonomous task runner: Jira → PRD → ralph → PR",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would be done without executing",
    )
    parser.add_argument(
        "--once",
        action="store_true",
        help="Run a single iteration then exit",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Enable debug logging",
    )
    parser.add_argument(
        "--max-iterations",
        type=int,
        default=None,
        help="Maximum iterations (0 = infinite, default from .env)",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> None:
    args = parse_args(argv)

    try:
        settings = Settings()  # type: ignore[call-arg]
    except Exception as exc:
        print(f"Configuration error: {exc}", file=sys.stderr)
        print("Ensure .env is present with required values. See .env.example.", file=sys.stderr)
        sys.exit(1)

    settings.dry_run = args.dry_run
    settings.verbose = args.verbose

    if args.once:
        settings.max_iterations = 1
    elif args.max_iterations is not None:
        settings.max_iterations = args.max_iterations

    setup_logging(verbose=settings.verbose)
    asyncio.run(run_loop(settings))


if __name__ == "__main__":
    main()
