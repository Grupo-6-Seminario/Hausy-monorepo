import {
  effect,
  frame,
  sampler,
  target,
  type Effect,
  type Gpu,
  type Surface,
  type Target,
} from 'vgpu';

import emitterWgsl from './emitter.wgsl';
import jfaInitWgsl from './jfa-init.wgsl';
import jfaPassWgsl from './jfa-pass.wgsl';
import presentWgsl from './present.wgsl';
import radianceCascadeWgsl from './radiance-cascade.wgsl';
import sdfFinalizeWgsl from './sdf-finalize.wgsl';

type Vec2 = readonly [number, number];

const HDR_FORMAT: GPUTextureFormat = 'rgba16float';
const SEED_FORMAT: GPUTextureFormat = 'rgba32float';
const RC_INTERVAL0 = 2;

export function createScene(gpu: Gpu, requestedSize: Vec2) {
  const size: Vec2 = [
    Math.max(1, Math.floor(requestedSize[0])),
    Math.max(1, Math.floor(requestedSize[1])),
  ];
  const cascadeCount = Math.min(
    5,
    Math.max(
      4,
      Math.ceil(
        Math.log(1 + (3 * Math.hypot(size[0], size[1])) / RC_INTERVAL0) / Math.log(4),
      ),
    ),
  );
  const spacing = 2 ** (cascadeCount - 1);
  const atlas: Vec2 = [
    Math.ceil(size[0] / spacing) * spacing * 2,
    Math.ceil(size[1] / spacing) * spacing * 2,
  ];
  const jumpCount = Math.ceil(Math.log2(Math.max(size[0], size[1], 2)));
  const jumps = [
    ...Array.from({ length: jumpCount }, (_, index) =>
      Math.max(1, 2 ** (jumpCount - index - 1)),
    ),
    1,
    1,
  ];
  const resources: Target[] = [];
  const own = (resource: Target) => {
    resources.push(resource);
    return resource;
  };

  const jfa: [Target, Target] = [
    own(target(gpu, { size, format: SEED_FORMAT })),
    own(target(gpu, { size, format: SEED_FORMAT })),
  ];
  const cascades: [Target, Target] = [
    own(target(gpu, { size: atlas, format: HDR_FORMAT })),
    own(target(gpu, { size: atlas, format: HDR_FORMAT })),
  ];

  return {
    gpu,
    size,
    atlas,
    cascadeCount,
    jumps,
    resources,
    emitter: own(target(gpu, { size, format: HDR_FORMAT })),
    jfa,
    sdf: own(target(gpu, { size, format: HDR_FORMAT })),
    cascades,
    effects: {
      emitter: effect(gpu, emitterWgsl),
      jfaInit: effect(gpu, jfaInitWgsl),
      jfaSteps: jumps.map(() => effect(gpu, jfaPassWgsl)),
      sdfFinalize: effect(gpu, sdfFinalizeWgsl),
      cascade: Array.from({ length: cascadeCount }, () =>
        effect(gpu, radianceCascadeWgsl),
      ),
      present: effect(gpu, presentWgsl, { blend: 'premultiplied' }),
    },
    sampler: sampler(gpu, {
      minFilter: 'linear',
      magFilter: 'linear',
      addressModeU: 'clamp-to-edge',
      addressModeV: 'clamp-to-edge',
    }),
  };
}

export type RadianceScene = ReturnType<typeof createScene>;

export async function prepareScene(scene: RadianceScene, outputFormat: GPUTextureFormat) {
  await Promise.all([
    scene.effects.emitter.compile({ colors: [HDR_FORMAT] }),
    scene.effects.jfaInit.compile({ colors: [SEED_FORMAT] }),
    ...scene.effects.jfaSteps.map((shader) => shader.compile({ colors: [SEED_FORMAT] })),
    scene.effects.sdfFinalize.compile({ colors: [HDR_FORMAT] }),
    ...scene.effects.cascade.map((shader) => shader.compile({ colors: [HDR_FORMAT] })),
    scene.effects.present.compile({ colors: [outputFormat] }),
  ]);
}

export function destroyScene(scene: RadianceScene) {
  for (let index = scene.resources.length - 1; index >= 0; index--) {
    (scene.resources[index] as Target & { destroy?: () => void }).destroy?.();
  }
}

export function runChain(scene: RadianceScene, time: number) {
  const passes: { target: Target; effect: Effect }[] = [];
  scene.effects.emitter.set({
    emitter: {
      state: [time, Math.min(scene.size[0], scene.size[1]) * 0.18, 10, 2.1],
      viewport: [scene.size[0], scene.size[1], 0, 0],
    },
  });
  passes.push({ target: scene.emitter, effect: scene.effects.emitter });

  scene.effects.jfaInit.set({ emitter: scene.emitter });
  passes.push({ target: scene.jfa[0], effect: scene.effects.jfaInit });

  let seedRead = scene.jfa[0];
  let seedWrite = scene.jfa[1];
  scene.jumps.forEach((jump, index) => {
    const shader = scene.effects.jfaSteps[index]!;
    shader.set({ jfa: { jump: [jump, 0, 0, 0] }, seeds: seedRead });
    passes.push({ target: seedWrite, effect: shader });
    [seedRead, seedWrite] = [seedWrite, seedRead];
  });

  scene.effects.sdfFinalize.set({ seeds: seedRead });
  passes.push({ target: scene.sdf, effect: scene.effects.sdfFinalize });

  let atlasWrite = scene.cascades[0];
  let atlasRead = scene.cascades[1];
  for (let cascade = scene.cascadeCount - 1; cascade >= 0; cascade--) {
    const shader = scene.effects.cascade[cascade]!;
    shader.set({
      rc: { state: [cascade, cascade < scene.cascadeCount - 1 ? 1 : 0, 0, 0] },
      sdf_tex: scene.sdf,
      sdf_samp: scene.sampler,
      emitter_tex: scene.emitter,
      emitter_samp: scene.sampler,
      upper_tex: atlasRead,
    });
    passes.push({ target: atlasWrite, effect: shader });
    [atlasRead, atlasWrite] = [atlasWrite, atlasRead];
  }
  scene.cascades = [atlasRead, atlasWrite];

  frame(scene.gpu, (currentFrame) => {
    for (const pass of passes) {
      currentFrame.pass({ target: pass.target, clear: [0, 0, 0, 0] }, (encoder) =>
        encoder.draw(pass.effect),
      );
    }
  });
}

export function presentScene(scene: RadianceScene, output: Surface) {
  scene.effects.present.set({
    cascade_tex: scene.cascades[0],
    emitter_tex: scene.emitter,
  });
  frame(scene.gpu, (currentFrame) => {
    currentFrame.pass({ target: output, clear: [0, 0, 0, 0] }, (encoder) =>
      encoder.draw(scene.effects.present),
    );
  });
}
