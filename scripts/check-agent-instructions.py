#!/usr/bin/env python3
"""Check instruction links, exhaustive skill routing, and baseline integrity."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import sys


def file_hashes(path):
    result = {}
    for item in sorted(path.rglob('*')):
        if item.is_file():
            data = item.read_bytes()
            normalized = data if b'\x00' in data else data.replace(b'\r\n', b'\n')
            result[item.relative_to(path).as_posix()] = hashlib.sha256(normalized).hexdigest()
    return result


def check(root, installed):
    failures = []
    manifest = json.loads((root / 'docs/agents/skills.lock.json').read_text(encoding='utf-8'))
    routing = (root / 'docs/agents/skills.md').read_text(encoding='utf-8')
    active = set(manifest['skills'])
    routed = set(re.findall(r'^\| `([^`]+)` \| (?:Required|Conditional|User invoked) \|', routing, re.M))
    if active != routed:
        failures.append(f'Routing mismatch: missing={active-routed}, extra={routed-active}')
    for name in manifest['removed']:
        if f'| `{name}` | Removed.' not in routing:
            failures.append(f'{name}: missing explicit removal decision')
        if installed:
            for base in ('.agents/skills', '.claude/skills'):
                path = root / base / name
                if path.exists() or path.is_symlink():
                    failures.append(f'{name}: removed skill remains at {base}')

    documents = [root / name for name in ('AGENTS.md', 'CLAUDE.md', 'GEMINI.md',
                                         'frontend/AGENTS.md', 'infra/AGENTS.md')]
    documents.extend((root / 'docs/agents').rglob('*.md'))
    for document in documents:
        if 'skill-overrides' in document.parts:
            continue  # Partial overlays resolve references after installation.
        text = re.sub(r'```.*?```', '', document.read_text(encoding='utf-8'), flags=re.S)
        for target in re.findall(r'(?<!!)\[[^\]]*\]\(([^\s)]+)\)', text):
            if '://' in target or target.startswith('#'):
                continue
            path = target.split('#', 1)[0]
            if path and not (document.parent / path).exists():
                failures.append(f'{document.relative_to(root)}: broken link {target}')
    if (root / 'frontend/DESIGN.md').exists():
        failures.append('frontend/DESIGN.md should be removed')

    for name, source in manifest['skills'].items():
        if installed:
            path = root / '.agents/skills' / name
        elif 'tracked' in source:
            path = root / source['tracked']
        elif 'override' in source and (root / source['override'] / 'SKILL.md').exists():
            path = root / source['override']
        elif 'snapshot' in source:
            path = root / source['snapshot']
        else:
            continue
        entry = path / 'SKILL.md'
        if not entry.exists():
            failures.append(f'{name}: missing {entry}')
            continue
        if hashlib.sha256(entry.read_bytes().replace(b'\r\n', b'\n')).hexdigest() != source['sha256']:
            failures.append(f'{name}: content differs from approved baseline')
        text = entry.read_text(encoding='utf-8')
        if not text.startswith('---\n') or 'description:' not in text.split('---', 2)[1]:
            failures.append(f'{name}: invalid skill frontmatter')
        if installed:
            if file_hashes(path) != source['files']:
                failures.append(f'{name}: installed supporting files differ from approved baseline')
            alias = root / '.claude/skills' / name / 'SKILL.md'
            if not alias.exists():
                failures.append(f'{name}: missing Claude skill alias or canonical entry')
            elif file_hashes(alias.parent) != source['files']:
                failures.append(f'{name}: Claude supporting files differ from approved baseline')
            for reference in path.rglob('*.md'):
                text = re.sub(r'```.*?```', '', reference.read_text(encoding='utf-8'), flags=re.S)
                text = re.sub(r'`[^`\n]+`', '', text)
                for target in re.findall(r'(?<!!)\[[^\]]*\]\(([^\s)]+)\)', text):
                    if '://' in target or target.startswith('#'):
                        continue
                    relative = target.split('#', 1)[0]
                    if relative and not (reference.parent / relative).exists():
                        failures.append(f'{name}/{reference.relative_to(path)}: broken skill pointer {target}')
    if installed:
        actual = {p.name for p in (root / '.agents/skills').iterdir() if (p / 'SKILL.md').exists()}
        if active - actual:
            failures.append(f'Installed baseline missing: {active-actual}')
        wrapper = root / manifest['worker_wrapper']
        expected = root / 'docs/agents/poteto-agent.md'
        if not wrapper.exists():
            failures.append('Missing project worker wrapper')
        elif wrapper.read_text(encoding='utf-8') != expected.read_text(encoding='utf-8'):
            failures.append('Project worker wrapper differs from approved source')
    if failures:
        print('\n'.join(failures), file=sys.stderr)
        return 1
    print(f'Instruction links, {len(active)} active skill routes, removal decisions, and checked hashes pass.')
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--installed', action='store_true', help='Also inspect installed skill contents and pointers')
    args = parser.parse_args()
    return check(Path(__file__).resolve().parents[1], args.installed)


if __name__ == '__main__':
    sys.exit(main())
