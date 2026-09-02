struct Params {
  time: f32,
  width: f32,
  height: f32,
}

@group(0) @binding(0) var<uniform> params: Params;

@fragment fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let aspect = params.width / max(params.height, 1.0);
  let centered = (uv - vec2f(0.5, 0.5)) * vec2f(aspect, 1.0);
  let drift = vec2f(sin(params.time * 0.12), cos(params.time * 0.09)) * 0.08;
  let first = exp(-4.5 * distance(centered, vec2f(0.32, -0.08) + drift));
  let second = exp(-5.4 * distance(centered, vec2f(-0.48, 0.22) - drift * 0.7));
  let ripple = sin((uv.x + uv.y) * 8.0 + params.time * 0.18) * 0.018;
  let base = vec3f(0.961, 0.976, 0.902);
  let mint = vec3f(0.435, 0.745, 0.569);
  let forest = vec3f(0.165, 0.353, 0.227);
  let color = mix(base, mint, first * 0.34 + ripple);
  color = mix(color, forest, second * 0.12);
  return vec4f(color, 0.78);
}
