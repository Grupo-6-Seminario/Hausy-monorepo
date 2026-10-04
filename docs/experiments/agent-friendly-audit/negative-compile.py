#!/usr/bin/env python3
"""Prove deleted interfaces and private authority fields reject unsupported callers."""
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
    ("legacy-search-port", "ddef7e4^", "ddef7e4", "internal/search", 'package search_test\nimport "github.com/Grupo-6-Seminario/proyecto-angus-back/internal/search"\nvar _ search.Repository\n'),
    ("parked-shortlist", "45d1f92^", "45d1f92", "internal/buyer", 'package buyer_test\nimport "github.com/Grupo-6-Seminario/proyecto-angus-back/internal/buyer"\nvar _ = buyer.BuildShortlist\n'),
]
env = {**os.environ, "GOPROXY": "off", "GOTOOLCHAIN": "local"}
results = []
for name, before, after, package, source in CASES:
    for label, revision in [("before", before), ("repaired", after), ("current", "HEAD")]:
        with tempfile.TemporaryDirectory(prefix="compile-", dir=OUT) as directory:
            target = Path(directory)
            archive_data = subprocess.check_output(["git", "archive", revision], cwd=ROOT)
            with tarfile.open(fileobj=io.BytesIO(archive_data)) as archive:
                archive.extractall(target, filter="data")
            (target / package / "audit_reference_test.go").write_text(source)
            command = ["go", "test", "./" + package, "-run", "^$", "-count=1"]
            run = subprocess.run(command, cwd=target, env=env, text=True, capture_output=True, timeout=60)
            results.append({"case": name, "revision": revision, "label": label, "command": command, "exit_code": run.returncode, "output": run.stdout + run.stderr})
with tempfile.TemporaryDirectory(prefix="compile-", dir=OUT) as directory:
    target = Path(directory)
    (target / "main.go").write_text('package main\nimport "github.com/Grupo-6-Seminario/proyecto-angus-back/docs/experiments/agent-friendly-audit/receipt"\nfunc main() { r, _ := receipt.New(nil); r.items = nil }\n')
    command = ["go", "build", "-o", str(target / "probe"), str(target / "main.go")]
    run = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=60)
    results.append({"case": "private-ranking-authority", "label": "prototype", "command": command[:3] + ["<temporary-output>", "<temporary-source>"], "exit_code": run.returncode, "output": run.stdout + run.stderr})
(OUT / "negative-compile-results.json").write_text(json.dumps(results, indent=2) + "\n")
for r in results:
    expected = 0 if r["label"] == "before" else 1
    print(f'{r["case"]}: {r["label"]}: exit {r["exit_code"]}')
    if r["exit_code"] != expected:
        raise SystemExit("unexpected compiler outcome")
