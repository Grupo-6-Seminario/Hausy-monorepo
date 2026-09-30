#!/usr/bin/env python3
"""Restore the pinned Hausy skill baseline without changing global installations."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def tree_digest(path):
    digest = hashlib.sha256()
    for item in sorted(path.rglob('*')):
        if item.is_file():
            digest.update(str(item.relative_to(path)).encode())
            data = item.read_bytes()
            digest.update(data if b'\x00' in data else data.replace(b'\r\n', b'\n'))
    return digest.hexdigest()


def file_hashes(path):
    result = {}
    for item in sorted(path.rglob('*')):
        if item.is_file():
            data = item.read_bytes()
            normalized = data if b'\x00' in data else data.replace(b'\r\n', b'\n')
            result[item.relative_to(path).as_posix()] = hashlib.sha256(normalized).hexdigest()
    return result


def install(root, refresh):
    manifest = json.loads((root / 'docs/agents/skills.lock.json').read_text(encoding='utf-8'))
    staged = {}
    with tempfile.TemporaryDirectory(prefix='hausy-skills-') as scratch:
        scratch = Path(scratch)
        repositories = {}
        for name, source in manifest['repositories'].items():
            checkout = scratch / name
            subprocess.run(['git', 'init', '-q', str(checkout)], check=True)
            subprocess.run(
                ['git', '-C', str(checkout), 'config', 'core.autocrlf', 'false'], check=True,
            )
            subprocess.run(
                ['git', '-C', str(checkout), 'fetch', '-q', '--depth', '1',
                 source['url'], source['revision']], check=True,
            )
            subprocess.run(
                ['git', '-C', str(checkout), 'checkout', '-q', '--detach', 'FETCH_HEAD'],
                check=True,
            )
            repositories[name] = checkout

        # Assemble and validate every source before changing the installed baseline.
        for name, source in manifest['skills'].items():
            if 'tracked' in source:
                origin = root / source['tracked']
            elif 'snapshot' in source:
                origin = root / source['snapshot']
            else:
                origin = repositories[source['repository']] / source['path']
            target = scratch / 'assembled' / name
            shutil.copytree(origin, target)
            if 'override' in source:
                shutil.copytree(root / source['override'], target, dirs_exist_ok=True)
            actual = hashlib.sha256(
                (target / 'SKILL.md').read_bytes().replace(b'\r\n', b'\n')
            ).hexdigest()
            if actual != source['sha256']:
                raise RuntimeError(f'{name}: assembled skill differs from approved baseline')
            assembled = file_hashes(target)
            if assembled != source['files']:
                missing = source['files'].keys() - assembled.keys()
                extra = assembled.keys() - source['files'].keys()
                changed = [p for p in assembled.keys() & source['files'].keys()
                           if assembled[p] != source['files'][p]]
                raise RuntimeError(
                    f'{name}: supporting files differ; missing={sorted(missing)}, '
                    f'extra={sorted(extra)}, changed={sorted(changed)}'
                )
            staged[name] = target

        for name, source in manifest['skills'].items():
            destinations = [root / '.agents/skills' / name]
            if 'tracked' not in source:
                destinations.append(root / '.claude/skills' / name)
            for destination in destinations:
                if destination.exists() and tree_digest(destination) != tree_digest(staged[name]):
                    if not refresh:
                        raise RuntimeError(
                            f'{destination}: differs; review local changes before --refresh'
                        )
        wrapper = root / manifest['worker_wrapper']
        expected_wrapper = root / 'docs/agents/poteto-agent.md'
        if wrapper.exists() and wrapper.read_text(encoding='utf-8') != expected_wrapper.read_text(encoding='utf-8') and not refresh:
            raise RuntimeError(f'{wrapper}: differs; review before --refresh')

        generated = []
        for name, source in manifest['skills'].items():
            canonical = root / '.agents/skills' / name
            replace_tree(canonical, staged[name])
            if 'tracked' not in source:
                alias = root / '.claude/skills' / name
                if not alias.exists() and not alias.is_symlink():
                    generated.append('/' + alias.relative_to(root).as_posix())
                if alias.is_symlink():
                    alias.unlink()
                elif alias.exists():
                    shutil.rmtree(alias)
                alias.parent.mkdir(parents=True, exist_ok=True)
                if os.name == 'nt':
                    shutil.copytree(canonical, alias)
                else:
                    alias.symlink_to(Path('../../.agents/skills') / name)

        wrapper.parent.mkdir(parents=True, exist_ok=True)
        if not wrapper.exists() and not wrapper.is_symlink():
            generated.append('/' + wrapper.relative_to(root).as_posix())
        if wrapper.is_symlink():
            wrapper.unlink()
        shutil.copy2(expected_wrapper, wrapper)
        exclude_generated(root, generated)
        for name in manifest['removed']:
            for base in (root / '.agents/skills', root / '.claude/skills'):
                path = base / name
                if path.is_symlink():
                    path.unlink()
                elif path.exists():
                    shutil.rmtree(path)
        print(f'Restored {len(staged)} pinned project skills. Global installations unchanged.')


def exclude_generated(root, paths):
    """Ignore newly generated aliases without hiding pre-existing personal files."""
    common = subprocess.check_output(
        ['git', '-C', str(root), 'rev-parse', '--path-format=absolute', '--git-common-dir'],
        text=True, encoding='utf-8',
    ).strip()
    exclude = Path(common) / 'info/exclude'
    exclude.parent.mkdir(parents=True, exist_ok=True)
    existing = exclude.read_text(encoding='utf-8') if exclude.exists() else ''
    additions = [path for path in paths if path not in existing.splitlines()]
    if additions:
        exclude.write_text(existing.rstrip() + '\n\n# Generated Hausy skill aliases\n' +
                           '\n'.join(additions) + '\n', encoding='utf-8')


def replace_tree(destination, source):
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination.exists() and tree_digest(destination) == tree_digest(source):
        return
    if destination.is_symlink():
        destination.unlink()
    elif destination.exists():
        shutil.rmtree(destination)
    shutil.copytree(source, destination)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--refresh', action='store_true', help='Replace reviewed project-local differences')
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    install(root, args.refresh)


if __name__ == '__main__':
    main()
