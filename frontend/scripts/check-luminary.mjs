import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { effect, frame, init, target } from 'vgpu/node';

// Render the actual WGSL through Dawn. Requires a native WebGPU adapter.
const gpu = await init();
const errors = [];
gpu.onError((error) => errors.push(error));
try {
  const source = readFileSync(
    new URL(
      '../app/components/prompt-luminary/prompt-luminary.wgsl',
      import.meta.url,
    ),
    'utf8',
  );
  const samples = [];
  for (const dpr of [1, 2]) {
    const width = 800 * dpr;
    const height = 300 * dpr;
    const output = target(gpu, {
      size: [width, height],
      format: 'rgba32float',
    });
    for (const state of ['rest', 'hover', 'searching']) {
      const shader = effect(gpu, source, {
        set: {
          luminary: {
            frame: [width, height, 0, state === 'searching' ? 1 : 0],
            pointer: [720 * dpr, 150 * dpr, state === 'hover' ? 1 : 0, 0],
            box: [400 * dpr, 150 * dpr, 320 * dpr, 70 * dpr],
            shape: [20 * dpr, dpr, 0, 0],
            deep_green: [0.2275, 0.4941, 0.3098, 1],
            cozy_green: [0.4353, 0.7451, 0.5686, 1],
            soft_green: [0.8275, 0.8784, 0.7216, 1],
          },
        },
      });
      frame(gpu, (current) =>
        current.pass({ target: output, clear: [0, 0, 0, 0] }, (pass) =>
          pass.draw(shader),
        ),
      );
      await gpu.settled();
      assert.deepEqual(
        errors,
        [],
        'GPU compilation and rendering must succeed',
      );
      const pixels = await output.readFloats();
      let peak = 0;
      let boundary = 0;
      let invalid = 0;
      let largestStep = 0;
      for (let y = 0; y < height; y++)
        for (let x = 0; x < width; x++) {
          const index = (y * width + x) * 4;
          const alpha = pixels[index + 3];
          peak = Math.max(peak, alpha);
          for (let channel = 0; channel < 4; channel++) {
            const value = pixels[index + channel];
            if (!Number.isFinite(value) || value < 0 || value > alpha + 0.00001)
              invalid++;
            if (x === 0 || x === width - 1 || y === 0 || y === height - 1)
              boundary = Math.max(boundary, value);
          }
          if (x > 0)
            largestStep = Math.max(
              largestStep,
              Math.abs(alpha - pixels[index - 1]),
            );
          if (y > 0)
            largestStep = Math.max(
              largestStep,
              Math.abs(alpha - pixels[index - width * 4 + 3]),
            );
        }
      assert.equal(
        invalid,
        0,
        'Premultiplied RGB must be finite, nonnegative, and <= alpha',
      );
      assert.equal(
        boundary,
        0,
        'Every canvas boundary must be fully transparent',
      );
      assert.ok(peak > 0.1, 'The halo must render visible light');
      assert.ok(
        largestStep < 0.04 / dpr,
        'Adjacent pixels must form a smooth gradient',
      );
      const alphaAt = (x, y) => pixels[(y * dpr * width + x * dpr) * 4 + 3];
      samples.push({
        state,
        side: alphaAt(723, 150),
        bottom: alphaAt(400, 228),
      });
      console.log({
        dpr,
        state,
        invalid,
        boundary,
        largestStep: +largestStep.toFixed(5),
      });
    }
  }
  for (const state of ['rest', 'hover', 'searching']) {
    const [one, two] = samples.filter((sample) => sample.state === state);
    assert.ok(
      Math.abs(one.side - two.side) < 0.01,
      'Retina must preserve halo size',
    );
    assert.ok(
      Math.abs(one.bottom - two.bottom) < 0.01,
      'Retina must preserve halo size',
    );
  }
  assert.ok(
    samples[1].side > samples[0].side * 1.5,
    'Hover must visibly brighten the nearby edge',
  );
  assert.ok(
    samples[2].bottom > samples[0].bottom * 1.3,
    'Searching must visibly brighten the halo',
  );
} finally {
  gpu.dispose();
}
