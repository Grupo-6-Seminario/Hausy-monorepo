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
    + length(max(corner, vec2f(0.0)))
    - safe_radius;
}

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let resolution = luminary.frame.xy;
  let time = luminary.frame.z;
  let is_active = luminary.frame.w;
  let hover = luminary.pointer.z;
  let pixel = uv * resolution;
  let distance_to_edge = rounded_rect_sdf(
    pixel - luminary.box.xy,
    luminary.box.zw,
    luminary.shape.x
  );

  // Every light band is derived from the control's real rounded perimeter.
  let breath = 0.94 + 0.06 * sin(time * 0.72);
  let edge = exp(-abs(distance_to_edge) / 2.4);
  let close_halo = exp(-abs(distance_to_edge) / 17.0);
  let wide_halo = exp(-abs(distance_to_edge) / 42.0);
  let pointer_distance = distance(pixel, luminary.pointer.xy);
  let pointer_focus = exp(-pointer_distance / 165.0) * hover;
  let resting_energy = (edge * 0.38 + close_halo * 0.18 + wide_halo * 0.045)
    * (0.64 * breath + 0.32 * is_active);
  let interactive_energy = (edge * 0.5 + close_halo * 0.34 + wide_halo * 0.08)
    * pointer_focus;
  let energy = resting_energy + interactive_energy;
  let warmth = clamp(edge * 0.18 + pointer_focus * 0.42, 0.0, 1.0);
  let base_colour = mix(luminary.deep_green.rgb, luminary.cozy_green.rgb, 0.72);
  let colour = mix(base_colour, luminary.soft_green.rgb, warmth);

  // Fade every edge before the canvas boundary to avoid visible rectangles.
  let edge_x = smoothstep(0.0, 0.09, uv.x) * smoothstep(0.0, 0.09, 1.0 - uv.x);
  let edge_y = smoothstep(0.0, 0.08, uv.y) * smoothstep(0.0, 0.08, 1.0 - uv.y);
  let containment = edge_x * edge_y;
  let mapped = (vec3f(1.0) - exp(-colour * energy * 2.25)) * containment;
  let alpha = clamp(max(mapped.r, max(mapped.g, mapped.b)) * 0.86, 0.0, 0.82);

  return vec4f(mapped, alpha);
}
