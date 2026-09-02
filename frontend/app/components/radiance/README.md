# Radiance-cascade backlight

This renderer is adapted from Vercel's verified [vgpu radiance-cascades example](https://vgpu.sh/examples/radiance-cascades), revision `90b65bf4144a6c18e275982fb1669336e3f9a1154ecbaac6174011f0c0ffeff1`.

The Angus adaptation keeps the example's jump-flood distance field, geometric radiance intervals, top-down cascade merge, HDR intermediate targets, and visibility alpha. It replaces pointer-painted emitters with a low-resolution animated rounded-border emitter and presents the light as a transparent green backlight behind the prompt box.

The adapted source remains under the MIT license in [LICENSE](./LICENSE).
