@group(0) @binding(0) var seeds: texture_2d<f32>;

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let size = vec2f(textureDimensions(seeds));
  let pixel = clamp(floor(uv * size), vec2f(0.0), size - 1.0);
  let seed = textureLoad(seeds, vec2i(pixel), 0);
  let distance_px = select(
    length(size) * 2.0,
    distance(seed.xy, pixel + 0.5),
    seed.w >= 0.5,
  );
  return vec4f(distance_px, 0.0, 0.0, 1.0);
}
