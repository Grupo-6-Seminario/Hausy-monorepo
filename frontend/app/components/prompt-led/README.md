# Prompt LED renderer

This renderer adapts vgpu's verified [triangle-led-front example](https://vgpu.sh/examples/triangle-led-front), revision `e5fc9af3626ca2d015fabe8a7f55311dd57c713e492150b4dd092b72cb4c94cc`.

The Angus version retains the example's analytic convex-edge lighting and discrete animated emitter model, but maps the emitters to the prompt's rounded rectangular perimeter. The WebGPU canvas is below an inset translucent surface, so the crisp LED edge and diffuse radiance originate behind the prompt instead of tinting its controls.

The upstream example and vgpu are MIT licensed; see `LICENSE`.
