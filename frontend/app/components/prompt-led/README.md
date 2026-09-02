# Prompt LED renderer

This renderer adapts vgpu's verified [triangle-led-front example](https://vgpu.sh/examples/triangle-led-front), revision `e5fc9af3626ca2d015fabe8a7f55311dd57c713e492150b4dd092b72cb4c94cc`.

The Hausy version retains the example's analytic convex-edge lighting and pointer-driven brightness lift, but maps the emitters to the prompt's rounded rectangular perimeter. The WebGPU canvas is below an inset translucent surface, so the crisp LED edge and diffuse radiance originate behind the prompt instead of tinting its controls.

Three adaptations are deliberate departures from upstream:

- **Arc-length parameterisation.** The perimeter coordinate is the distance travelled around the shape rather than an angle, so the shimmer moves at a constant speed instead of stalling along the long edges and racing around the corners.
- **No facing test.** Upstream only lights an emitter whose normal points towards the cursor. Here the cursor spends most of its time *inside* the shape, where every normal points away, so proximity alone drives the lift.
- **One continuous emitter.** Upstream addresses LEDs individually. At this size the gaps between them read as dark notches rather than as a strip, and quantised brightness smeared along the outward normal bands the halo into rectangles, so the perimeter is unbroken and only breathes slightly. Hovering warms it rather than switching it into a brighter mode.

The pointer is tracked on the window — the canvas itself is `pointer-events: none` and sits under the prompt — and eased with the same 0.23 s time constant as upstream's `glowSmoothing`.

The upstream example and vgpu are MIT licensed; see `LICENSE`.
