export const SDF_HIT_EPSILON: f32 = 0.5;
export const SDF_MIN_STEP: f32 = 0.35;
export const SDF_MAX_STEPS: i32 = 16;

export fn sdf_pixel_uv(pixel: vec2f, size: vec2f) -> vec2f {
  let half_texel = 0.5 / size;
  return clamp(pixel / size, half_texel, vec2f(1.0) - half_texel);
}

export fn sphere_trace(
  sdf_tex: texture_2d<f32>,
  sdf_samp: sampler,
  emitter_tex: texture_2d<f32>,
  emitter_samp: sampler,
  size: vec2f,
  origin: vec2f,
  direction: vec2f,
  t_start: f32,
  t_end: f32,
) -> vec4f {
  var t = t_start;
  for (var step = 0; step < SDF_MAX_STEPS; step = step + 1) {
    let point = origin + direction * t;
    if (
      point.x < -1.0 || point.y < -1.0 ||
      point.x > size.x + 1.0 || point.y > size.y + 1.0
    ) { break; }
    let distance_to_emitter = textureSampleLevel(
      sdf_tex,
      sdf_samp,
      sdf_pixel_uv(point, size),
      0.0,
    ).r;
    if (distance_to_emitter <= SDF_HIT_EPSILON) {
      let emitter = textureSampleLevel(
        emitter_tex,
        emitter_samp,
        sdf_pixel_uv(point, size),
        0.0,
      );
      return vec4f(emitter.rgb, 0.0);
    }
    t = t + max(distance_to_emitter, SDF_MIN_STEP);
    if (t > t_end) { break; }
  }
  return vec4f(0.0, 0.0, 0.0, 1.0);
}
