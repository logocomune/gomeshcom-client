import { createCanvasContext2D } from 'ol/dom';
import { getBottomRight, getTopLeft } from 'ol/extent';
import ImageLayer from 'ol/layer/Image';
import { fromLonLat, toLonLat } from 'ol/proj';
import ImageCanvas from 'ol/source/ImageCanvas';
import type { Extent } from 'ol/extent';
import type Projection from 'ol/proj/Projection';
import type { Size } from 'ol/size';
import { getLocator } from './maidenhead';

const FONT_FAMILY = 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

type GridConstants = {
	lonStep: number;
	latStep: number;
	precision: number;
	fontSize: number;
};

export function getMaidenheadLayer(): ImageLayer<ImageCanvas> {
	return new ImageLayer({
		source: new ImageCanvas({
			canvasFunction: drawGrid
		}),
		opacity: 0.9
	});
}

function drawGrid(
	extent: Extent,
	resolution: number,
	pixelRatio: number,
	size: Size,
	projection: Projection
): HTMLCanvasElement {
	const constants = constantsForResolution(resolution);
	const context = createCanvasContext2D(size[0], size[1]) as CanvasRenderingContext2D;
	const rp = resolution / pixelRatio;
	const offset = [extent[0] / rp, extent[3] / rp];
	const topLeft = toLonLat(getTopLeft(extent), projection);
	const bottomRight = toLonLat(getBottomRight(extent), projection);

	const startLon = roundDown(topLeft[0], constants.lonStep);
	const endLon = roundUp(bottomRight[0], constants.lonStep);
	const startLat = roundUp(topLeft[1], constants.latStep);
	const endLat = roundDown(bottomRight[1], constants.latStep);

	for (let lon = startLon; lon <= endLon; lon += constants.lonStep) {
		for (let lat = startLat; lat >= endLat; lat -= constants.latStep) {
			drawLine(context, projection, [lon, lat], [lon, lat - constants.latStep], offset, rp);
			drawLine(context, projection, [lon, lat], [lon + constants.lonStep, lat], offset, rp);
			drawLabel(context, projection, constants, lon, lat, offset, rp, pixelRatio);
		}
	}

	return context.canvas;
}

function constantsForResolution(resolution: number): GridConstants {
	if (resolution < 120) return { lonStep: 1 / 12, latStep: 1 / 24, precision: 3, fontSize: 13 };
	if (resolution < 2500) return { lonStep: 2, latStep: 1, precision: 2, fontSize: 20 };
	return { lonStep: 20, latStep: 10, precision: 1, fontSize: 28 };
}

function drawLine(
	context: CanvasRenderingContext2D,
	projection: Projection,
	from: number[],
	to: number[],
	offset: number[],
	rp: number
): void {
	const start = fromLonLat(from, projection);
	const end = fromLonLat(to, projection);

	context.beginPath();
	context.moveTo(start[0] / rp - offset[0], offset[1] - start[1] / rp);
	context.lineTo(end[0] / rp - offset[0], offset[1] - end[1] / rp);
	context.strokeStyle = 'rgba(239,68,68,0.58)';
	context.setLineDash([7, 4]);
	context.lineWidth = 1;
	context.stroke();
}

function drawLabel(
	context: CanvasRenderingContext2D,
	projection: Projection,
	constants: GridConstants,
	lon: number,
	lat: number,
	offset: number[],
	rp: number,
	pixelRatio: number
): void {
	const centerLon = lon + constants.lonStep / 2;
	const centerLat = lat - constants.latStep / 2;
	const center = fromLonLat([centerLon, centerLat], projection);
	const x = center[0] / rp - offset[0];
	const y = offset[1] - center[1] / rp + 4 * pixelRatio;

	const isField = constants.precision === 1;
	context.font = `${isField ? '700' : '600'} ${constants.fontSize}px ${FONT_FAMILY}`;
	context.textAlign = 'center';
	context.textBaseline = 'middle';
	context.fillStyle = isField ? 'rgba(127,29,29,0.98)' : 'rgba(153,27,27,0.88)';
	context.strokeStyle = isField ? 'rgba(255,255,255,0.92)' : 'rgba(255,255,255,0.86)';
	context.lineWidth = isField ? 6 : 4;
	context.lineJoin = 'round';
	const text = getLocator(centerLon, centerLat, constants.precision);
	context.strokeText(text, x, y);
	context.fillText(text, x, y);
}

function roundUp(value: number, step: number): number {
	return roundDown(value, step) + step;
}

function roundDown(value: number, step: number): number {
	let offset = value % step;
	if (offset < 0) offset += step;
	return value - offset;
}
