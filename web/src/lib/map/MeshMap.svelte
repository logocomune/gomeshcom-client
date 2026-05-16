<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import 'ol/ol.css';
	import { getMaidenheadLayer } from './maidenhead-layer';
	import type { MapPosition } from './types';
	import { nodeFreshness, FRESHNESS_FILL, FRESHNESS_ZINDEX } from './node-state';

	let { positions = [] }: { positions?: MapPosition[] } = $props();

	let mapElement: HTMLDivElement;
	let tooltipElement: HTMLDivElement;
	let map: any;
	let markerSource: any;
	let nightLayer: any;
	let maidenheadLayer: any;
	let showNight = $state(false);
	let showMaidenhead = $state(true);
	let initialized = $state(false);
	let selectedPosition = $state<MapPosition | null>(null);

	$effect(() => {
		positions;
		if (initialized) updateMarkers();
	});

	onMount(async () => {
		const [
			{ Map, View },
			{ Tile: TileLayer, Vector: VectorLayer, Image: ImageLayer },
			{ OSM, Vector: VectorSource, ImageCanvas },
			{ fromLonLat },
			{ Style, Fill, Stroke, Circle: CircleStyle, Text },
			Feature,
			{ Point },
			Overlay
		] = await Promise.all([
			import('ol'),
			import('ol/layer'),
			import('ol/source'),
			import('ol/proj'),
			import('ol/style'),
			import('ol/Feature').then((module) => module.default),
			import('ol/geom'),
			import('ol/Overlay').then((module) => module.default)
		]);

		const dayNightModule = await import('ol-ext/source/DayNight.js');
		const DayNight = dayNightModule.default;
		const dayNight = new DayNight();

		markerSource = new VectorSource();
		const markerLayer = new VectorLayer({ source: markerSource });

		maidenheadLayer = getMaidenheadLayer();
		maidenheadLayer.setVisible(showMaidenhead);

		const nightSource = new ImageCanvas({
			canvasFunction: (extent: number[], resolution: number, pixelRatio: number, size: number[]) => {
				const canvas = document.createElement('canvas');
				canvas.width = size[0] * pixelRatio;
				canvas.height = size[1] * pixelRatio;
				const context = canvas.getContext('2d');
				if (!context) return canvas;

				const coords: number[][] = dayNight.getCoordinates(new Date());
				context.save();
				context.filter = `blur(${Math.min(Math.max(Math.round(400_000 / (resolution * pixelRatio)), 8), 80)}px)`;
				context.beginPath();
				for (const [index, coordinate] of coords.entries()) {
					const [lon, lat] = coordinate;
					const projected = fromLonLat([lon, Math.max(-85, Math.min(85, lat))]);
					const x = ((projected[0] - extent[0]) / (extent[2] - extent[0])) * canvas.width;
					const y = ((extent[3] - projected[1]) / (extent[3] - extent[1])) * canvas.height;
					if (index === 0) context.moveTo(x, y);
					else context.lineTo(x, y);
				}
				context.closePath();
				context.fillStyle = 'rgba(15,23,42,0.58)';
				context.fill();
				context.restore();
				return canvas;
			},
			projection: 'EPSG:3857',
			ratio: 1
		});

		nightLayer = new ImageLayer({
			source: nightSource,
			opacity: 0,
			visible: true
		});

		map = new Map({
			target: mapElement,
			controls: [],
			layers: [
				new TileLayer({ source: new OSM() }),
				maidenheadLayer,
				nightLayer,
				markerLayer
			],
			view: new View({
				center: fromLonLat([12.5, 42.5]),
				zoom: 6,
				maxZoom: 19
			})
		});

		const tooltip = new Overlay({
			element: tooltipElement,
			offset: [12, 12],
			positioning: 'top-left'
		});
		map.addOverlay(tooltip);

		map.on('pointermove', (event: any) => {
			const feature = map.forEachFeatureAtPixel(event.pixel, (candidate: any) => candidate);
			if (!feature) {
				tooltip.setPosition(undefined);
				tooltipElement.classList.add('hidden');
				return;
			}

			const position = feature.get('position') as MapPosition;
			tooltipElement.textContent = `${position.source} · ${position.lat.toFixed(4)}, ${position.lon.toFixed(4)}`;
			tooltipElement.classList.remove('hidden');
			tooltip.setPosition(event.coordinate);
		});

		map.on('click', (event: any) => {
			const feature = map.forEachFeatureAtPixel(event.pixel, (candidate: any) => candidate);
			selectedPosition = feature?.get('position') ?? null;
		});

		initialized = true;
		updateMarkers();

		function updateMarkers() {
			if (!markerSource) return;
			markerSource.clear();
			const now = Date.now();
			for (const position of positions) {
				const freshness = nodeFreshness(position, now);
				if (freshness === 'hidden') continue;
				const feature = new Feature({
					geometry: new Point(fromLonLat([position.lon, position.lat])),
					position
				});
				feature.setStyle(
					new Style({
						zIndex: FRESHNESS_ZINDEX[freshness],
						image: new CircleStyle({
							radius: 6,
							fill: new Fill({ color: FRESHNESS_FILL[freshness] }),
							stroke: new Stroke({ color: '#ecfeff', width: 2 })
						}),
						text: new Text({
							text: position.source,
							font: '11px Inter, sans-serif',
							offsetY: -16,
							fill: new Fill({ color: '#f9fafb' }),
							stroke: new Stroke({ color: '#111827', width: 3 })
						})
					})
				);
				markerSource.addFeature(feature);
			}
		}
	});

	onDestroy(() => {
		map?.setTarget(undefined);
	});

	function updateMarkers() {
		// Replaced by onMount closure once OpenLayers classes are loaded.
	}

	function zoomBy(delta: number) {
		const view = map?.getView();
		if (!view) return;
		view.animate({ zoom: (view.getZoom() ?? 6) + delta, duration: 180 });
	}

	function recenter() {
		const view = map?.getView();
		if (!view) return;
		view.animate({ center: view.getProjection() ? undefined : undefined, zoom: 6, duration: 250 });
		if (positions.length > 0) {
			const latest = positions[0];
			import('ol/proj').then(({ fromLonLat }) => {
				view.animate({ center: fromLonLat([latest.lon, latest.lat]), zoom: 8, duration: 400 });
			});
		}
	}

	function toggleNight() {
		showNight = !showNight;
		nightLayer?.setOpacity(showNight ? 1 : 0);
	}

	function toggleMaidenhead() {
		showMaidenhead = !showMaidenhead;
		maidenheadLayer?.setVisible(showMaidenhead);
	}
</script>

<div class="relative h-full w-full overflow-hidden bg-[#0d1017]">
	<div bind:this={mapElement} class="h-full w-full"></div>

	<div class="absolute left-2 top-2 z-[1000] flex flex-col rounded shadow-md">
		<button class="h-7 w-7 rounded-t border-b border-gray-300 bg-white font-bold text-gray-800 hover:bg-gray-100" onclick={() => zoomBy(1)}>+</button>
		<button class="h-7 w-7 border-b border-gray-300 bg-white font-bold text-gray-800 hover:bg-gray-100" onclick={() => zoomBy(-1)}>-</button>
		<button
			class="h-7 w-7 border-b border-gray-300 bg-white text-xs font-bold text-gray-800 hover:bg-gray-100 {showMaidenhead ? 'text-red-600' : 'text-gray-400'}"
			title="Toggle Maidenhead grid"
			onclick={toggleMaidenhead}
		>
			M
		</button>
		<button
			class="h-7 w-7 rounded-b bg-white text-xs font-bold text-gray-800 hover:bg-gray-100 {showNight ? 'text-blue-600' : 'text-gray-400'}"
			title="Toggle day/night zone"
			onclick={toggleNight}
		>
			◐
		</button>
	</div>

	<button
		class="absolute right-2 top-2 z-[1000] rounded border border-gray-300 bg-white px-2 py-1 text-xs font-semibold text-gray-800 shadow hover:bg-gray-100"
		onclick={recenter}
	>
		Recenter
	</button>

	<div class="absolute bottom-2 left-2 z-[1000] rounded border border-gray-700 bg-[#1e2330]/90 px-2 py-1 font-mono text-[11px] text-gray-200">
		{positions.length} positions · OSM · Maidenhead
	</div>

	<div class="absolute bottom-2 right-2 z-[1000] rounded border border-gray-700 bg-[#1e2330]/90 px-2 py-1 text-[10px] text-gray-400">
		© OpenStreetMap contributors
	</div>

	<div
		bind:this={tooltipElement}
		class="pointer-events-none absolute z-[2000] hidden rounded border border-gray-700 bg-gray-950 px-2 py-1 text-[11px] font-semibold text-white shadow-md"
	></div>

	{#if selectedPosition}
		<div class="absolute right-2 top-12 z-[1000] w-64 rounded border border-gray-700 bg-[#1e2330]/95 p-3 shadow-xl">
			<div class="flex items-center justify-between">
				<div class="truncate text-sm font-semibold text-white">{selectedPosition.source}</div>
				<button class="text-gray-400 hover:text-gray-100" onclick={() => (selectedPosition = null)}>×</button>
			</div>
			<div class="mt-2 space-y-1 font-mono text-[11px] text-gray-300">
				<div>{selectedPosition.lat.toFixed(5)}, {selectedPosition.lon.toFixed(5)}</div>
				{#if selectedPosition.battery != null}<div>batt {selectedPosition.battery}%</div>{/if}
				{#if selectedPosition.rssi != null}<div>rssi {selectedPosition.rssi} dBm</div>{/if}
				{#if selectedPosition.snr != null}<div>snr {selectedPosition.snr}</div>{/if}
			</div>
		</div>
	{/if}
</div>
