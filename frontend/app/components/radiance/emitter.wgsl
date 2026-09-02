// Angus emitter adapted from vgpu's radiance-cascades example.
// A softly animated rounded border becomes the light source for the cascade chain.

struct Emitter {
  /** x: time, y: inset, z: corner radius, w: intensity. */
  state: vec4f,
  /** xy: render target size. */
  viewport: vec4f,
};

@group(0) @binding(0) var<uniform> emitter: Emitter;

fn rounded_box_distance(point: vec2f, half_size: vec2f, radius: f32) -> f32 {
  let q = abs(point) - half_size + radius;
  return min(max(q.x, q.y), 0.0) + length(max(q, vec2f(0.0))) - radius;
}

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let size = emitter.viewport.xy;
  let pixel = uv * size;
  let centre = size * 0.5;
  let inset = emitter.state.y;
  let half_size = max(vec2f(12.0), centre - vec2f(inset));
  let distance_to_border = abs(
    rounded_box_distance(pixel - centre, half_size, emitter.state.z),
  );
  let border = 1.0 - smoothstep(0.7, 2.8, distance_to_border);
  let travel = 0.5 + 0.5 * sin(
    pixel.x * 0.032 + pixel.y * 0.018 + emitter.state.x * 0.72,
  );
  let pulse = 0.78 + travel * 0.22;
  let forest = vec3f(0.027, 0.102, 0.044);
  let mint = vec3f(0.159, 0.515, 0.283);
  let radiance = mix(forest, mint, travel) * emitter.state.w * pulse;
  return vec4f(radiance * border, border);
}
