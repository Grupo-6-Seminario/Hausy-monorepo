#!/usr/bin/env python3
"""Replay selected historical regressions without changing the checkout."""
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[3]
OUT = Path(__file__).resolve().parent
CASES = [
    ("sale-provenance", "40bb5d9", "internal/intake/planner_test.go", "./internal/intake", "TestASaleTheUserNeverStatedIsARent"),
    ("writer-example-facts", "256298a", "internal/buyer/writer_test.go", "./internal/buyer", "TestLocalWriterExamplesCarryNoCountOrGuaranteeAModelCouldCopy"),
    ("amenity-withdrawal", "876dc31", "internal/intake/qwen_test.go", "./internal/intake", "TestQwenDropsAnAttributeValueOutsideTheVocabulary"),
    # The pre-fix test already specifies the observable outcome, without referring to the newly introduced type.
    ("clarification-fallback", "087b641", "internal/intake/planner_test.go", "./internal/intake", "TestUnnamedAmenitiesAreNotAPlannerFailure"),
    ("database-startup", "958b332", "cmd/hausy/main_test.go", "./cmd/hausy", "TestServerRefusesToStartWithoutItsDatabase"),
    ("credentials-startup", "73ef67c", "cmd/hausy/main_test.go", "./cmd/hausy", "TestServerRefusesToStartWithoutBedrockCredentials"),
]


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT)


def main():
    results = []
    env = {**os.environ, "GOPROXY": "off", "GOTOOLCHAIN": "local"}
    for name, fix, path, package, test in CASES:
        test_revision = fix + "^" if name == "clarification-fallback" else fix
        frozen_test = git("show", f"{test_revision}:{path}")
        for label, revision in [("before", fix + "^"), ("repaired", fix), ("current", "HEAD")]:
            with tempfile.TemporaryDirectory(prefix="replay-", dir=OUT) as directory:
                target = Path(directory)
                # All entries come from this repository's tracked Git tree.
                with tarfile.open(fileobj=io.BytesIO(git("archive", revision))) as archive:
                    archive.extractall(target, filter="data")
                # Keep the old test and its supporting fixtures consistent. Current tests are run separately.
                if label != "current":
                    (target / path).write_bytes(frozen_test)
                command = ["go", "test", package, "-run", "^" + test + "$", "-count=1", "-v"]
                run = subprocess.run(command, cwd=target, env=env, text=True, capture_output=True, timeout=60)
                result = {"case": name, "label": label, "revision": git("rev-parse", revision).decode().strip(), "test_source": test_revision if label != "current" else "HEAD", "command": command, "exit_code": run.returncode, "output": run.stdout + run.stderr}
                results.append(result)
                print(f"{name}: {label}: exit {run.returncode}", flush=True)
    (OUT / "replay-results.json").write_text(json.dumps(results, indent=2) + "\n")
    if any(r["exit_code"] == 0 for r in results if r["label"] == "before") or any(r["exit_code"] != 0 for r in results if r["label"] != "before"):
        raise SystemExit("A replay did not have the expected fail/pass/pass sequence; inspect replay-results.json")


if __name__ == "__main__":
    main()
