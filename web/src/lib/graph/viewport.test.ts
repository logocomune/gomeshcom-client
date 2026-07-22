import { describe, expect, it } from 'vitest';
import fc from 'fast-check';
import { initialViewport, panViewport, zoomViewport, type GraphViewport } from './viewport';

describe('graph viewport', () => {
	it('starts with full layout bounds', () => {
		expect(initialViewport(800, 600)).toEqual({ x: 0, y: 0, width: 800, height: 600 });
	});

	it('zooms around anchor point', () => {
		const viewport: GraphViewport = { x: 10, y: 20, width: 400, height: 200 };
		const zoomed = zoomViewport(viewport, { x: 0.25, y: 0.5 }, 2, { width: 800, height: 600 });

		expect(zoomed).toEqual({ x: 60, y: 70, width: 200, height: 100 });
	});

	it('pans opposite to drag direction in graph coordinates', () => {
		const viewport: GraphViewport = { x: 0, y: 0, width: 400, height: 200 };
		const panned = panViewport(viewport, { x: 100, y: -50 }, { width: 400, height: 200 });

		expect(panned).toEqual({ x: -100, y: 50, width: 400, height: 200 });
	});

	it('keeps zoom anchor stable in graph coordinates', () => {
		fc.assert(
			fc.property(
				fc.record({
					x: fc.double({ min: -1000, max: 1000, noNaN: true }),
					y: fc.double({ min: -1000, max: 1000, noNaN: true }),
					width: fc.double({ min: 300, max: 5000, noNaN: true }),
					height: fc.double({ min: 300, max: 5000, noNaN: true })
				}),
				fc.record({
					x: fc.double({ min: 0, max: 1, noNaN: true }),
					y: fc.double({ min: 0, max: 1, noNaN: true })
				}),
				fc.double({ min: 0.75, max: 2, noNaN: true }),
				(viewport, anchor, factor) => {
					const before = {
						x: viewport.x + viewport.width * anchor.x,
						y: viewport.y + viewport.height * anchor.y
					};
					const afterViewport = zoomViewport(viewport, anchor, factor, {
						width: viewport.width,
						height: viewport.height
					});
					const after = {
						x: afterViewport.x + afterViewport.width * anchor.x,
						y: afterViewport.y + afterViewport.height * anchor.y
					};

					expect(after.x).toBeCloseTo(before.x, 8);
					expect(after.y).toBeCloseTo(before.y, 8);
				}
			),
			{ numRuns: 100 }
		);
	});
});
