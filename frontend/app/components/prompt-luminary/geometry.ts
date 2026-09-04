export interface RectLike {
  left: number;
  top: number;
  width: number;
  height: number;
}

export interface PromptBoxMetrics {
  box: [number, number, number, number];
  radius: number;
}

export function promptBoxMetrics(
  canvas: RectLike,
  control: RectLike,
  renderSize: readonly [number, number],
  radiusCssPixels: number,
): PromptBoxMetrics {
  const scaleX = renderSize[0] / Math.max(canvas.width, 1);
  const scaleY = renderSize[1] / Math.max(canvas.height, 1);

  return {
    box: [
      (control.left - canvas.left + control.width / 2) * scaleX,
      (control.top - canvas.top + control.height / 2) * scaleY,
      (control.width / 2) * scaleX,
      (control.height / 2) * scaleY,
    ],
    radius: radiusCssPixels * Math.min(scaleX, scaleY),
  };
}
