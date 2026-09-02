// Adapted from vgpu's triangle-led-front example: analytic edge emitters are
// distributed over a convex perimeter, then composed as a crisp front LED line
// plus diffuse radiance. The convex shape here is the prompt's rounded rectangle.
struct Config {
  frame: vec4f,
  shape: vec4f,
  colour_a: vec4f,
  colour_b: vec4f,
};

@group(0) @binding(0) var<uniform> led: Config;

fn rounded_box_sdf(point: vec2f, half_size: vec2f, radius: f32) -> f32 {
  let q = abs(point) - half_size + vec2f(radius);
  return length(max(q, vec2f(0.0))) + min(max(q.x, q.y), 0.0) - radius;
}

fn perimeter_phase(point: vec2f, half_size: vec2f) -> f32 {
  let normalized = point / max(half_size, vec2f(1.0));
  let angle = atan2(normalized.y, normalized.x);
  return (angle + 3.14159265) / 6.2831853;
}

fn tonemap(colour: vec3f) -> vec3f {
  return colour / (colour + vec3f(1.0));
}

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let resolution = led.frame.xy;
  let point = uv * resolution - resolution * 0.5;
  let half_size = max(resolution * 0.5 - vec2f(led.frame.w), vec2f(12.0));
  let distance = rounded_box_sdf(point, half_size, led.shape.x);

  // Discrete emitters travel along the perimeter, mirroring triangle-led-front's
  // individually animated edge LEDs without putting a canvas above the prompt.
  let phase = perimeter_phase(point, half_size);
  let segment = floor(phase * led.shape.z);
  let led_wave = 0.58 + 0.42 * sin(segment * 1.618 - led.frame.z * 1.45);
  let travelling = 0.62 + 0.38 * sin(phase * 12.56637 - led.frame.z * 0.72);
  let emitter_strength = max(0.16, led_wave * travelling);
  let hue = 0.5 + 0.5 * sin(phase * 6.2831853 - led.frame.z * 0.18);
  let emitter_colour = mix(led.colour_a.rgb, led.colour_b.rgb, hue);

  let front = exp(-abs(distance) / max(led.shape.y, 0.5)) * emitter_strength;
  let outside = max(distance, 0.0);
  let backlight = exp(-outside * 0.052) * smoothstep(-3.0, 1.0, distance);
  let halo = exp(-outside * 0.018) * smoothstep(0.0, 9.0, outside) * 0.2;
  let energy = emitter_colour * (front * 2.35 + backlight * 0.66 + halo);
  let mapped = tonemap(energy);
  let alpha = clamp(front * 0.96 + backlight * 0.48 + halo * 0.35, 0.0, 0.92);
  return vec4f(mapped * alpha, alpha);
}
