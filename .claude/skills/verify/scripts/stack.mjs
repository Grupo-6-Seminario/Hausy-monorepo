#!/usr/bin/env node
// Start, check and stop the whole Hausy stack for a live drive:
//
//	node .claude/skills/verify/scripts/stack.mjs up|doctor|down
//
// The servers, ports and environment come from
// frontend/playwright.journeys.config.ts, so this script and the journeys
// always run the same stack. `up` seeds the disposable hausy_e2e_test
// database, reuses any server already answering, and records what it started
// in frontend/test-results/verify/stack.json; `down` stops only that.
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import { register } from 'node:module';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const frontend = path.join(root, 'frontend');
const runDir = path.join(frontend, 'test-results/verify');
const stateFile = path.join(runDir, 'stack.json');

// The config imports './journeys/fixture' without an extension, as Playwright
// allows; Node strips the types but needs the .ts suffix to find the file.
register(
  'data:text/javascript,' +
    encodeURIComponent(`export async function resolve(specifier, context, next) {
  try { return await next(specifier, context); }
  catch (error) {
    if (specifier.startsWith('.') && !/\\.[cm]?[jt]s$/.test(specifier)) return next(specifier + '.ts', context);
    throw error;
  }
}`),
);
const config = (await import(path.join(frontend, 'playwright.journeys.config.ts'))).default;
const servers = config.webServer.map((server, i) => ({
  name: ['fakemodel', 'api', 'frontend'][i] ?? `server-${i}`,
  ...server,
}));

// Playwright counts a server as up when its URL answers 2xx, 3xx or 400-403.
async function answers(url) {
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(2000) });
    return response.status < 404;
  } catch {
    return false;
  }
}

function portOpen(port) {
  return new Promise((resolve) => {
    const socket = net.connect(port, '127.0.0.1');
    socket.once('connect', () => {
      socket.end();
      resolve(true);
    });
    socket.once('error', () => resolve(false));
  });
}

function alive(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function readState() {
  return fs.existsSync(stateFile) ? JSON.parse(fs.readFileSync(stateFile, 'utf8')) : null;
}

function run(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit' });
  if (result.status !== 0) throw new Error(`${command} ${args.join(' ')} exited ${result.status}`);
}

async function doctor() {
  let healthy = true;
  for (const server of servers) {
    const ok = await answers(server.url);
    healthy &&= ok;
    console.log(`${ok ? 'ok  ' : 'DOWN'} ${server.name.padEnd(9)} ${server.url}`);
  }
  const db = await portOpen(5432);
  healthy &&= db;
  console.log(`${db ? 'ok  ' : 'DOWN'} postgres  localhost:5432`);
  for (const started of readState()?.started ?? []) {
    if (!alive(started.pid)) {
      healthy = false;
      console.log(`DEAD ${started.name} pid ${started.pid}; log: ${started.log}`);
    }
  }
  return healthy;
}

async function down() {
  const state = readState();
  if (!state) {
    console.log('nothing recorded as started; nothing to stop');
    return;
  }
  for (const started of state.started) {
    // Each server runs in its own process group, so `go run` and `npm run`
    // children stop with it.
    try {
      process.kill(-started.pid, 'SIGTERM');
    } catch {}
  }
  const deadline = Date.now() + 10_000;
  while (state.started.some((s) => alive(s.pid)) && Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 200));
  }
  for (const started of state.started.filter((s) => alive(s.pid))) {
    try {
      process.kill(-started.pid, 'SIGKILL');
    } catch {}
  }
  if (state.postgres === 'stop') run('docker', ['compose', 'stop', 'postgres'], root);
  if (state.postgres === 'remove container') run('docker', ['compose', 'down'], root);
  if (state.postgres === 'remove container and volume') run('docker', ['compose', 'down', '--volumes'], root);
  fs.rmSync(stateFile);
  console.log(`stopped ${state.started.map((s) => s.name).join(', ') || 'no servers'}${state.postgres ? `; postgres: ${state.postgres}` : ''}; logs and evidence stay in ${runDir}`);
}

async function up() {
  if (readState()) {
    if (await doctor()) {
      console.log('already up');
      return;
    }
    throw new Error(`a previous run is recorded in ${stateFile} and is unhealthy; run down first`);
  }
  fs.mkdirSync(path.join(runDir, 'logs'), { recursive: true });
  const state = { postgres: null, started: [], reused: [] };
  const save = () => fs.writeFileSync(stateFile, JSON.stringify(state, null, 2));
  save();
  try {
    if (!(await portOpen(5432))) {
      // Remember what Docker had, so down returns it to exactly that.
      const exists = (args) => spawnSync('docker', args, { cwd: root, encoding: 'utf8' }).stdout.trim() !== '';
      state.postgres = exists(['compose', 'ps', '-a', '-q', 'postgres'])
        ? 'stop'
        : exists(['volume', 'ls', '-q', '--filter', 'name=^hausy_hausy-postgres-data$'])
          ? 'remove container'
          : 'remove container and volume';
      save();
      run('docker', ['compose', 'up', '-d', '--wait', 'postgres'], root);
    }
    // Drops and recreates hausy_e2e_test; the seed refuses any other name.
    run('go', ['run', '../e2e/journeys/seed'], frontend);
    for (const server of servers) {
      if (await answers(server.url)) {
        state.reused.push(server.url);
        console.log(`reusing ${server.name} at ${server.url}`);
        save();
        continue;
      }
      const log = path.join(runDir, 'logs', `${server.name}.log`);
      const out = fs.openSync(log, 'w');
      const child = spawn(server.command, {
        shell: true,
        cwd: frontend,
        detached: true,
        env: { ...process.env, ...server.env },
        stdio: ['ignore', out, out],
      });
      child.unref();
      state.started.push({ name: server.name, pid: child.pid, url: server.url, log });
      save();
      console.log(`starting ${server.name}; log: ${log}`);
      const deadline = Date.now() + server.timeout;
      while (!(await answers(server.url))) {
        if (!alive(child.pid) || Date.now() > deadline) {
          throw new Error(`${server.name} did not answer at ${server.url}; see ${log}`);
        }
        await new Promise((r) => setTimeout(r, 500));
      }
    }
    console.log(`up: open ${config.use.baseURL}`);
  } catch (error) {
    await down();
    throw error;
  }
}

const command = process.argv[2];
try {
  if (command === 'up') await up();
  else if (command === 'down') await down();
  else if (command === 'doctor') process.exitCode = (await doctor()) ? 0 : 1;
  else {
    console.error('usage: stack.mjs up|doctor|down');
    process.exitCode = 2;
  }
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
