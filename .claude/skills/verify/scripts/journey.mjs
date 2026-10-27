#!/usr/bin/env node
// The representative drive, and the template for new ones: a new searcher
// signs up, declares a qualification, finds a listing that accepts it, and
// opens the listings that refuse it. Each step saves a screenshot.
//
//	node .claude/skills/verify/scripts/journey.mjs
//
// Needs `stack.mjs up` first. Evidence goes to
// frontend/test-results/verify/<timestamp>/; the script exits non-zero at the
// first step whose expected text is missing, after saving that step's screenshot.
import { execSync } from 'node:child_process';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';

// The repository root, wherever a copy of this script sits inside it.
const root = execSync('git rev-parse --show-toplevel', { cwd: import.meta.dirname, encoding: 'utf8' }).trim();
const frontend = path.join(root, 'frontend');
const { chromium, expect } = createRequire(path.join(frontend, 'package.json'))('@playwright/test');
// fixture.ts also exports the seeded realtor and listings; import what you use.
const { hydrated, ports, search } = await import(path.join(frontend, 'journeys/fixture.ts'));

const evidence = path.join(frontend, 'test-results/verify', new Date().toISOString().replace(/[:.]/g, '-'));
fs.mkdirSync(evidence, { recursive: true });

const browser = await chromium.launch({ channel: 'chrome' });
const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, baseURL: `http://127.0.0.1:${ports.frontend}` });
const errors = [];
page.on('pageerror', (error) => errors.push(error.message));

let shot = 0;
async function step(name, body) {
  try {
    await body();
    console.log(`ok   ${name}`);
  } finally {
    // Animations disabled, so cards are captured settled, not mid-fade.
    await page.screenshot({ path: path.join(evidence, `${++shot}-${name.replaceAll(' ', '-')}.png`), fullPage: true, animations: 'disabled' });
  }
}

// capture saves one element on its own: a full-page shot repeats the sticky
// header and side panel over content further down.
async function capture(locator, name) {
  // Centered, so the sticky header never covers the element's top.
  await locator.evaluate((element) => element.scrollIntoView({ block: 'center' }));
  await locator.screenshot({ path: path.join(evidence, `${shot + 1}-${name.replaceAll(' ', '-')}.png`), animations: 'disabled' });
}

try {
  await step('sign up', async () => {
    await page.goto('/ingresar');
    await page.getByRole('tab', { name: 'Crear cuenta' }).click();
    await page.getByLabel('Nombre').fill('Verify');
    await page.getByLabel('Email').fill(`verify-${Date.now()}@journeys.test`);
    await page.getByLabel('Contraseña').fill('verify-searcher');
    await page.getByRole('button', { name: 'Crear cuenta' }).click();
    await page.getByRole('link', { name: 'Completar mi perfil y buscar' }).click();
    await hydrated(page);
  });

  await step('declare qualification', async () => {
    const answer = (fact, choice) => page.getByRole('group', { name: fact }).getByText(choice).click();
    await answer('Garantía', 'Garantía propietaria');
    await answer('¿Podés comprobar tus ingresos?', 'Sí');
    await answer('Mascotas', 'Ninguna');
    await page.getByLabel('Ingresos mensuales').selectOption({ label: 'Más de $3.000.000' });
    const use = page.getByRole('button', { name: 'Usar estos datos' });
    await use.click();
    await expect(use).toBeHidden();
  });

  await step('eligible in Palermo', async () => {
    await search(page, /¿Qué estás buscando\?/, 'Busco alquiler en Palermo');
    const card = page.getByRole('article').filter({ hasText: 'Soler 5818' });
    await expect(card).toContainText('Cumplís lo que pide');
    await capture(card, 'soler-card');
  });

  await step('refused in Monserrat', async () => {
    await search(page, /Sumá a tu búsqueda/, 'Busco alquiler en Monserrat');
    const refused = page.getByRole('button', { name: 'Ver las 3 propiedades que no aceptan tu situación' });
    await expect(refused).toBeVisible();
    await refused.click();
    const card = page.getByRole('article').filter({ hasText: 'Moreno al 400' });
    await expect(card).toContainText('No calificás');
    await capture(card, 'moreno-card');
  });

  if (errors.length) throw new Error(`page errors: ${errors.join('; ')}`);
  console.log(`evidence: ${evidence}`);
} catch (error) {
  console.error(`FAIL ${error.message.split('\n')[0]}\nevidence: ${evidence}`);
  process.exitCode = 1;
} finally {
  await browser.close();
}
