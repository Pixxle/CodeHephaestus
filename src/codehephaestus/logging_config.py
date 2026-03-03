from __future__ import annotations

import logging
import sys


def setup_logging(*, verbose: bool = False) -> None:
    level = logging.DEBUG if verbose else logging.INFO
    fmt = "[%(asctime)s] %(levelname)-5s %(name)s: %(message)s"
    datefmt = "%Y-%m-%d %H:%M:%S"

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(logging.Formatter(fmt, datefmt=datefmt))

    root = logging.getLogger("codehephaestus")
    root.setLevel(level)
    root.addHandler(handler)
    root.propagate = False
