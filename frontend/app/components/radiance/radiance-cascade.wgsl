import {
  rc_atlas_decode,
  rc_atlas_texel,
  rc_block_size,
  rc_direction,
  rc_probe_origin,
  rc_probe_spacing,
  rc_ray_count,
} from "./rc-directions.wgsl";
import { sphere_trace } from "./sdf-sample.wgsl";

fn interval_start(cascade: f32) -> f32 {
  return 2.0 * (pow(4.0, cascade) - 1.0) / 3.0;
}

fn interval_end(cascade: f32) -> f32 {
  return interval_start(cascade) + 2.0 * pow(4.0, cascade) * 1.02;
}

fn merge_radiance(near: vec4f, far: vec4f) -> vec4f {
  return vec4f(near.rgb + near.a * far.rgb, near.a * far.a);
}

fn bilinear_weights(fraction: vec2f) -> vec4f {
  let f = clamp(fraction, vec2f(0.0), vec2f(1.0));
  return vec4f(
    (1.0 - f.x) * (1.0 - f.y),
    f.x * (1.0 - f.y),
    (1.0 - f.x) * f.y,
    f.x * f.y,
  );
}

struct Cascade {
  /** x: level index, y: upper-cascade-present flag. */
  state: vec4f,
};

@group(0) @binding(0) var<uniform> rc: Cascade;
@group(0) @binding(1) var sdf_tex: texture_2d<f32>;
@group(0) @binding(2) var sdf_samp: sampler;
@group(0) @binding(3) var emitter_tex: texture_2d<f32>;
@group(0) @binding(4) var emitter_samp: sampler;
@group(0) @binding(5) var upper_tex: texture_2d<f32>;

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let atlas_size = vec2f(textureDimensions(upper_tex));
  let scene_size = vec2f(textureDimensions(sdf_tex));
  let cascade = rc.state.x;
  let block = rc_block_size(cascade);
  let decoded = rc_atlas_decode(floor(uv * atlas_size), block);
  let spacing = rc_probe_spacing(cascade);
  let origin = rc_probe_origin(decoded.xy, spacing);
  let direction = rc_direction(decoded.z, rc_ray_count(cascade));

  var radiance = sphere_trace(
    sdf_tex,
    sdf_samp,
    emitter_tex,
    emitter_samp,
    scene_size,
    origin,
    direction,
    interval_start(cascade),
    interval_end(cascade),
  );

  if (rc.state.y > 0.5) {
    let upper_block = block * 2.0;
    let upper_spacing = spacing * 2.0;
    let upper_grid = atlas_size / upper_block;
    let position = origin / upper_spacing - 0.5;
    let base = floor(position);
    let weights = bilinear_weights(position - base);
    var weight_array = array<f32, 4>(weights.x, weights.y, weights.z, weights.w);
    var far = vec4f(0.0);

    for (var branch = 0; branch < 4; branch = branch + 1) {
      let upper_direction = decoded.z * 4.0 + f32(branch);
      var interpolated = vec4f(0.0);
      for (var corner = 0; corner < 4; corner = corner + 1) {
        let offset = vec2f(f32(corner % 2), f32(corner / 2));
        let neighbour = clamp(base + offset, vec2f(0.0), upper_grid - 1.0);
        let coord = rc_atlas_texel(neighbour, upper_direction, upper_block);
        interpolated += weight_array[corner] * textureLoad(upper_tex, vec2i(coord), 0);
      }
      far += interpolated * 0.25;
    }
    radiance = merge_radiance(radiance, far);
  }

  return radiance;
}
