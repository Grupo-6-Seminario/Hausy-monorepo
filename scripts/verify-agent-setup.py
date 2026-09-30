#!/usr/bin/env python3
"""Exercise fresh skill setup and its known failure paths in a disposable checkout."""

import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


def run(root, script, *args, succeeds=True):
    result = subprocess.run(
        [sys.executable, str(root / 'scripts' / script), *args],
        capture_output=True, text=True, encoding='utf-8',
    )
    if (result.returncode == 0) != succeeds:
        raise RuntimeError(result.stdout + result.stderr)
    return result.stdout + result.stderr


def installed_state(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in (root / '.agents/skills').rglob('*') if p.is_file()}


def verify(source, root):
    paths = ['.gitignore', 'AGENTS.md', 'CLAUDE.md', 'GEMINI.md', 'CONTEXT.md', 'frontend/AGENTS.md',
             'infra/AGENTS.md', 'docs/agents', 'docs/SETUP.md', 'docs/DATA_MODEL.md',
             'docs/metrics', 'docs/MATCHING_CONTRACT.md', 'docs/AGENCY_CATALOG.md', 'docs/adr', 'scripts',
             '.claude/skills/amazon-bedrock', '.claude/skills/deslop',
             '.claude/skills/ship', '.claude/skills/test-audit']
    for relative in paths:
        origin, target = source / relative, root / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        if origin.is_dir():
            shutil.copytree(origin, target)
        else:
            shutil.copy2(origin, target)
    subprocess.run(['git', 'init', '-q', str(root)], check=True)
    run(root, 'setup-agent-skills.py')
    run(root, 'check-agent-instructions.py', '--installed')
    before = installed_state(root)
    run(root, 'setup-agent-skills.py')
    if installed_state(root) != before:
        raise RuntimeError('Repeated setup changed skill contents')
    print('Fresh install and repeated install pass.', flush=True)

    alias = root / '.claude/skills/ponytail'
    alias.unlink() if alias.is_symlink() else shutil.rmtree(alias)
    failure = run(root, 'check-agent-instructions.py', '--installed', succeeds=False)
    if 'missing Claude' not in failure:
        raise RuntimeError('Missing alias failed for the wrong reason')
    shutil.copytree(root / '.agents/skills/ponytail', alias)
    wrapper = root / '.claude/agents/poteto-agent.md'
    wrapper.unlink()
    failure = run(root, 'check-agent-instructions.py', '--installed', succeeds=False)
    if 'Missing project worker wrapper' not in failure:
        raise RuntimeError('Missing worker wrapper failed for the wrong reason')
    shutil.copy2(root / 'docs/agents/poteto-agent.md', wrapper)
    print('Missing alias and wrapper are detected.', flush=True)

    support = root / '.agents/skills/tdd/tests.md'
    original = support.read_bytes()
    support.write_text(support.read_text(encoding='utf-8') + '\nUnreviewed supporting-file change.\n', encoding='utf-8')
    failure = run(root, 'check-agent-instructions.py', '--installed', succeeds=False)
    if 'installed supporting files differ' not in failure:
        raise RuntimeError('Supporting-file drift failed for the wrong reason')
    support.write_bytes(original)
    alias = root / '.claude/skills/tdd'
    if alias.is_symlink():
        alias.unlink()
        shutil.copytree(root / '.agents/skills/tdd', alias)
    alias_support = alias / 'tests.md'
    alias_support.unlink()
    failure = run(root, 'check-agent-instructions.py', '--installed', succeeds=False)
    if 'Claude supporting files differ' not in failure:
        raise RuntimeError('Missing Claude supporting file failed for the wrong reason')
    shutil.copy2(support, alias_support)
    print('Changed or missing supporting files are detected.', flush=True)

    manifest_file = root / 'docs/agents/skills.lock.json'
    manifest_text = manifest_file.read_text(encoding='utf-8')
    manifest = json.loads(manifest_text)
    support.write_text(support.read_text(encoding='utf-8') + '\n[Missing](missing-guide.md)\n', encoding='utf-8')
    shutil.copy2(support, alias_support)
    manifest['skills']['tdd']['files']['tests.md'] = hashlib.sha256(support.read_bytes()).hexdigest()
    manifest_file.write_text(json.dumps(manifest), encoding='utf-8')
    failure = run(root, 'check-agent-instructions.py', '--installed', succeeds=False)
    if 'broken skill pointer missing-guide.md' not in failure or 'supporting files differ' in failure:
        raise RuntimeError('Supporting link validation did not independently detect the broken pointer')
    support.write_bytes(original)
    shutil.copy2(support, alias_support)
    manifest_file.write_text(manifest_text, encoding='utf-8')
    print('Broken supporting-document links fail even with matching content hashes.', flush=True)

    personal = root / '.agents/skills/personal-helper'
    personal.mkdir()
    (personal / 'SKILL.md').write_text(
        '---\nname: personal-helper\ndescription: Personal local helper.\n---\n\nLocal content.\n'
    , encoding='utf-8')
    run(root, 'check-agent-instructions.py', '--installed')
    entry = root / '.claude/skills/deslop/SKILL.md'
    entry.write_bytes(entry.read_bytes().replace(b'\r\n', b'\n').replace(b'\n', b'\r\n'))
    run(root, 'check-agent-instructions.py', '--installed')
    run(root, 'setup-agent-skills.py')
    run(root, 'check-agent-instructions.py', '--installed')
    if not (personal / 'SKILL.md').exists():
        raise RuntimeError('Setup removed an unrelated personal skill')
    print('Personal skills are preserved and CRLF text passes.', flush=True)

    changed = root / '.agents/skills/ponytail/SKILL.md'
    changed.write_text(changed.read_text(encoding='utf-8') + '\nUnreviewed local change.\n', encoding='utf-8')
    before = installed_state(root)
    failure = run(root, 'setup-agent-skills.py', succeeds=False)
    if 'review local changes before --refresh' not in failure or installed_state(root) != before:
        raise RuntimeError('Drift rejection failed or modified installed files')
    print('Unreviewed drift is rejected before installed files change.', flush=True)

    manifest = json.loads((root / 'docs/agents/skills.lock.json').read_text(encoding='utf-8'))
    generated = [root / '.claude/skills' / name for name, spec in manifest['skills'].items()
                 if 'tracked' not in spec]
    generated.append(wrapper)
    for path in generated:
        result = subprocess.run(['git', '-C', str(root), 'check-ignore', '-q', str(path)])
        if result.returncode != 0:
            raise RuntimeError(f'Generated path is not ignored: {path}')
    print('Generated Claude aliases and wrapper are Git-ignored.', flush=True)


if __name__ == '__main__':
    with tempfile.TemporaryDirectory(prefix='hausy-checkout-') as directory:
        verify(Path(__file__).resolve().parents[1], Path(directory))
    print('All setup behavior checks passed; disposable resources cleaned up.')
