"""Run the shared feature files with Python bindings, without copying them."""

import sys
from pathlib import Path
from tempfile import TemporaryDirectory

from behave.__main__ import main

HERE = Path(__file__).resolve().parent
SHARED = HERE.parents[1] / "features"

with TemporaryDirectory(prefix="tuicast-behave-") as temporary:
    features = Path(temporary) / "features"
    features.mkdir()
    for feature in SHARED.glob("*.feature"):
        (features / feature.name).symlink_to(feature)
    (features / "steps").symlink_to(HERE / "steps", target_is_directory=True)
    (features / "environment.py").symlink_to(HERE / "steps/environment.py")
    arguments = [str(features), "--format=json.pretty"]
    if not any(argument.startswith("--outfile") for argument in sys.argv[1:]):
        arguments.append("--outfile=cucumber-report.json")
    raise SystemExit(main([*arguments, *sys.argv[1:]]))
