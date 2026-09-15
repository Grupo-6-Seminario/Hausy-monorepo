struct Luminary {
  frame: vec4f,
  pointer: vec4f,
  box: vec4f,
  shape: vec4f,
  deep_green: vec4f,
  cozy_green: vec4f,
  soft_green: vec4f,
};
@group(0) @binding(0) var<uniform> luminary: Luminary;

fn rounded_rect_sdf(point: vec2f, half_size: vec2f, radius: f32) -> f32 {
  let safe_radius = min(radius, min(half_size.x, half_size.y));
  let corner = abs(point) - half_size + vec2f(safe_radius);
  return min(max(corner.x, corner.y), 0.0)
    + length(max(corner, vec2f(0.0))) - safe_radius;
}

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  // Distances are CSS pixels: Retina changes resolution, not the light's size.
  let scale = max(luminary.shape.y, 1.0);
  let resolution = luminary.frame.xy / scale;
  let pixel = uv * resolution;
  let center = luminary.box.xy / scale;
  let half_size = luminary.box.zw / scale;
  let time = luminary.frame.z;
  let is_working = luminary.frame.w;
  let proximity = luminary.pointer.z;
  let sdf = rounded_rect_sdf(pixel - center, half_size, luminary.shape.x / scale);
  let outside = max(sdf, 0.0);

  // Broad Gaussian falloff has no hard ring or narrow, aliased peak.
  let near_light = exp(-0.5 * pow(outside / 12.0, 2.0));
  let atmosphere = exp(-0.5 * pow(outside / 27.0, 2.0));
  let pointer_distance = distance(pixel, luminary.pointer.xy / scale);
  let pointer_light = exp(-0.5 * pow(pointer_distance / 115.0, 2.0)) * proximity;
  let wave = 0.5 + 0.5 * sin(pixel.x * 0.009 - time * 0.8);
  let underlight = smoothstep(-half_size.y, half_size.y + 14.0, pixel.y - center.y);
  let energy = near_light * (0.11 + underlight * 0.11 + pointer_light * 0.30)
    + atmosphere * (0.055 + underlight * 0.05 + is_working * (0.13 + wave * 0.10));

  // A fixed 32 CSS-pixel feather reaches zero before every canvas edge.
  let margin = min(pixel, resolution - pixel);
  let containment = smoothstep(1.0, 32.0, margin.x) * smoothstep(1.0, 32.0, margin.y);
  let alpha = clamp(energy * containment, 0.0, 0.7);
  let colour = mix(luminary.deep_green.rgb, luminary.cozy_green.rgb, 0.75 + pointer_light * 0.25);
  // vgpu surfaces use premultiplied alpha. RGB must vanish with alpha.
  return vec4f(colour * alpha, alpha);
}
