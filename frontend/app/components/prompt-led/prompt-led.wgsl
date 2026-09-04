// Adapted from vgpu's triangle-led-front example: emitters are distributed at
// even arc-length intervals over a convex perimeter, brighten towards the
// pointer (radius + facing test, as upstream's brush glow does), and are then
// composed as a crisp front LED line plus diffuse radiance. The convex shape
// here is the prompt's rounded rectangle.
struct Config {
  frame: vec4f,
  shape: vec4f,
  pointer: vec4f,
  colour_a: vec4f,
  colour_b: vec4f,
  colour_c: vec4f,
};

@group(0) @binding(0) var<uniform> led: Config;

const TAU = 6.2831853;
const HALF_PI = 1.5707963;

struct Boundary {
  point: vec2f,
  normal: vec2f,
  distance: f32,
  phase: f32,
};

// Arc length of the nearest perimeter point, measured clockwise from the start
// of the top edge. Even spacing here is what keeps the emitters from bunching
// up at the corners the way an angular parameterisation does.
fn arc_phase(core: vec2f, normal: vec2f, straight: vec2f, radius: f32) -> f32 {
  let quarter = HALF_PI * radius;
  let total = 4.0 * straight.x + 4.0 * straight.y + 4.0 * quarter;
  let on_x = abs(core.x) >= straight.x - 0.001;
  let on_y = abs(core.y) >= straight.y - 0.001;
  var s = 0.0;
  if (on_x && on_y) {
    if (core.x > 0.0 && core.y < 0.0) {
      s = 2.0 * straight.x + atan2(normal.x, -normal.y) * radius;
    } else if (core.x > 0.0) {
      s = 2.0 * straight.x + quarter + 2.0 * straight.y + atan2(normal.y, normal.x) * radius;
    } else if (core.y > 0.0) {
      s = 4.0 * straight.x + 2.0 * quarter + 2.0 * straight.y + atan2(-normal.x, normal.y) * radius;
    } else {
      s = 4.0 * straight.x + 3.0 * quarter + 4.0 * straight.y + atan2(-normal.y, -normal.x) * radius;
    }
  } else if (!on_x) {
    if (core.y < 0.0) {
      s = core.x + straight.x;
    } else {
      s = 2.0 * straight.x + 2.0 * quarter + 2.0 * straight.y + (straight.x - core.x);
    }
  } else {
    if (core.x > 0.0) {
      s = 2.0 * straight.x + quarter + (core.y + straight.y);
    } else {
      s = 4.0 * straight.x + 3.0 * quarter + 2.0 * straight.y + (straight.y - core.y);
    }
  }
  return fract(s / max(total, 1.0));
}

fn boundary_of(point: vec2f, half_size: vec2f, radius: f32) -> Boundary {
  let straight = max(half_size - vec2f(radius), vec2f(0.0));
  let core = clamp(point, -straight, straight);
  let offset = point - core;
  let reach = length(offset);
  let normal = select(vec2f(0.0, -1.0), offset / reach, reach > 0.0001);
  var out: Boundary;
  out.normal = normal;
  out.point = core + normal * radius;
  out.distance = reach - radius;
  out.phase = arc_phase(core, normal, straight, radius);
  return out;
}

fn tonemap(colour: vec3f) -> vec3f {
  return vec3f(1.0) - exp(-colour);
}

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let resolution = led.frame.xy;
  let time = led.frame.z;
  let inset = led.frame.w;
  let point = uv * resolution - resolution * 0.5;
  let half_size = max(resolution * 0.5 - vec2f(inset) + vec2f(led.shape.w), vec2f(12.0));
  let edge = boundary_of(point, half_size, led.shape.x);

  // One continuous emitter around the perimeter. The upstream example drives
  // individually addressed LEDs; discrete emitters read as dark gaps at this
  // scale, so the strip is unbroken and only breathes very slightly.
  let shimmer = 1.0 - led.shape.z * (0.5 - 0.5 * sin(edge.phase * TAU * 2.0 - time * 0.55));

  // Pointer glow: emitters within reach of the cursor run hot. Upstream also
  // gates on the emitter facing the pointer, which is dropped here on purpose —
  // the cursor spends most of its time *inside* this shape, where every normal
  // points away and that test would blank the whole strip.
  // The pointer arrives in canvas space; the perimeter is centred on the canvas.
  let pointer = led.pointer.xy - resolution * 0.5;
  let pointer_reach = length(pointer - edge.point);
  let proximity = 1.0 - smoothstep(0.0, max(led.pointer.w, 1.0), pointer_reach);
  let lift = clamp(led.pointer.z * proximity, 0.0, 1.0);
  // Hovering only warms the strip; it is not a second, brighter mode.
  let emitter = shimmer * (1.0 + 0.5 * lift);

  // The border is the sole chromatic artifact in the interface: its phase
  // travels through the three approved prism channels while pointer proximity
  // changes energy, never the palette.
  let colour_phase = fract(edge.phase + time * 0.006);
  let red_to_cyan = mix(led.colour_a.rgb, led.colour_b.rgb, smoothstep(0.0, 0.5, colour_phase));
  let emitter_colour = mix(red_to_cyan, led.colour_c.rgb, smoothstep(0.5, 1.0, colour_phase));

  let outside = max(edge.distance, 0.0);
  let front = exp(-abs(edge.distance) / max(led.shape.y, 0.5));
  // Radiance escapes outwards only: without this gate the interior, where
  // `outside` is pinned to zero, would sit at full brightness.
  let outward = smoothstep(-6.0, 1.5, edge.distance);
  let bloom = exp(-outside / max(inset * 0.16, 1.0)) * outward;
  let halo = exp(-outside / max(inset * 0.45, 1.0)) * outward;
  // The canvas has to end somewhere; fade the tail out before its edge so the
  // radiance never terminates in a visible rectangle.
  let containment = 1.0 - smoothstep(inset * 0.45, inset * 0.99, outside);

  let energy = emitter_colour * emitter * (front * 0.92 + bloom * 0.44 + halo * 0.176)
    + vec3f(front * front * emitter * 0.128);
  let mapped = tonemap(energy) * containment;
  // Emissive output: the premultiplied colour is the light itself and alpha is
  // just its coverage, so the glow adds to the page instead of veiling it.
  let alpha = clamp(max(mapped.r, max(mapped.g, mapped.b)), 0.0, 1.0);
  return vec4f(mapped, alpha);
}
