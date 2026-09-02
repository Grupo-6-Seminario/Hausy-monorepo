import { rc_atlas_texel, rc_block_size, rc_ray_count } from "./rc-directions.wgsl";

@group(0) @binding(0) var cascade_tex: texture_2d<f32>;
@group(0) @binding(1) var emitter_tex: texture_2d<f32>;

fn resolve_cascade0(pixel: vec2f) -> vec3f {
  let block = rc_block_size(0.0);
  let rays = rc_ray_count(0.0);
  let atlas_size = vec2f(textureDimensions(cascade_tex));
  let probe = clamp(floor(pixel), vec2f(0.0), atlas_size / block - 1.0);
  var total = vec3f(0.0);
  for (var index = 0.0; index < rays; index = index + 1.0) {
    total += textureLoad(
      cascade_tex,
      vec2i(rc_atlas_texel(probe, index, block)),
      0,
    ).rgb;
  }
  return total / rays;
}

fn tonemap(color: vec3f) -> vec3f {
  return color / (color + vec3f(1.0));
}

@fragment
fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let size = vec2f(textureDimensions(emitter_tex));
  let pixel = uv * size;
  let texel = vec2i(clamp(floor(pixel), vec2f(0.0), size - 1.0));
  let irradiance = resolve_cascade0(pixel);
  let direct = textureLoad(emitter_tex, texel, 0);
  let energy = tonemap(irradiance * 0.7 + direct.rgb * 0.45);
  let luminance = dot(energy, vec3f(0.2126, 0.7152, 0.0722));
  let depth = clamp(luminance * 1.9, 0.0, 1.0);
  let forest = vec3f(0.165, 0.353, 0.227);
  let mint = vec3f(0.435, 0.745, 0.569);
  let colour = mix(forest, mint, depth);
  let alpha = smoothstep(0.015, 0.34, luminance) * 0.72;
  return vec4f(colour * alpha, alpha);
}
