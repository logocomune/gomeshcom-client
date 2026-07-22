export type GraphViewport = {
	x: number;
	y: number;
	width: number;
	height: number;
};

export type Point = {
	x: number;
	y: number;
};

export type Size = {
	width: number;
	height: number;
};

const MIN_SIZE = 80;
const MAX_SIZE_MULTIPLIER = 4;

export function initialViewport(width: number, height: number): GraphViewport {
	return { x: 0, y: 0, width, height };
}

export function zoomViewport(
	viewport: GraphViewport,
	anchor: Point,
	factor: number,
	maxSize: Size
): GraphViewport {
	const nextWidth = clamp(viewport.width / factor, MIN_SIZE, maxSize.width * MAX_SIZE_MULTIPLIER);
	const nextHeight = clamp(
		viewport.height / factor,
		MIN_SIZE,
		maxSize.height * MAX_SIZE_MULTIPLIER
	);
	const anchorX = clamp(anchor.x, 0, 1);
	const anchorY = clamp(anchor.y, 0, 1);

	return {
		x: viewport.x + (viewport.width - nextWidth) * anchorX,
		y: viewport.y + (viewport.height - nextHeight) * anchorY,
		width: nextWidth,
		height: nextHeight
	};
}

export function panViewport(
	viewport: GraphViewport,
	deltaPixels: Point,
	viewportPixels: Size
): GraphViewport {
	if (viewportPixels.width <= 0 || viewportPixels.height <= 0) return viewport;

	return {
		...viewport,
		x: viewport.x - (deltaPixels.x / viewportPixels.width) * viewport.width,
		y: viewport.y - (deltaPixels.y / viewportPixels.height) * viewport.height
	};
}

function clamp(value: number, min: number, max: number): number {
	return Math.min(Math.max(value, min), max);
}
