#!/usr/bin/env python3
"""Run preserved independent controls without adding files to application packages."""
import json
from pathlib import Path
import subprocess
import tempfile

artifacts = Path(__file__).resolve().parent
root = artifacts.parents[2]
with tempfile.TemporaryDirectory(prefix="hausy-oracle-") as scratch:
    overlay = Path(scratch) / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {
        str(root / "experiments/property-matching/oracle_controls_test.go"): str(artifacts / "oracle-controls-final.go.txt"),
        str(root / "internal/buyer/oracle_baseline_test.go"): str(artifacts / "baseline-controls.go.txt"),
    }}))
    subprocess.run(["go", "test", "-overlay", str(overlay), "./experiments/property-matching", "./internal/buyer", "-run", "TestOracle", "-v", "-count=1"], cwd=root, check=True)
