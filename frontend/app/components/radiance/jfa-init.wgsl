@group(0) @binding(0) var emitter: texture_2d<f32>;

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let size = vec2f(textureDimensions(emitter));
  let pixel = clamp(floor(uv * size), vec2f(0.0), size - 1.0);
  let mask = textureLoad(emitter, vec2i(pixel), 0).a;
  return select(
    vec4f(0.0),
    vec4f(pixel + 0.5, 0.0, 1.0),
    mask > 0.5,
  );
}
