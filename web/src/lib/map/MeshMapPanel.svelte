<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import 'ol/ol.css';
	import {
		mdiCrosshairsGps,
		mdiGrid,
		mdiGridOff,
		mdiLayersTriple,
		mdiLayersTripleOutline,
		mdiMapMarkerPath,
		mdiMinus,
		mdiPlus,
		mdiRuler,
		mdiTagOff,
		mdiTagText
	} from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { getMaidenheadLayer } from './maidenhead-layer';
	import type { StreamEvent } from '$lib/api/types';
	import type { MapPosition } from './types';
	import {
		nodeFreshness,
		FRESHNESS_FILL,
		FRESHNESS_ZINDEX,
		MYCALL_FILL,
		MYCALL_ZINDEX
	} from './node-state';
	import { buildOwnMarkerTooltipHtml, buildTooltipHtml, escHtml } from './map-tooltip';
	import { buildRulerLinks } from './ruler';
	import { buildRealtimeDmTraceSegments } from './realtime-trace';

	const STORAGE_CENTER = 'meshcom:map:center';
	const STORAGE_ZOOM = 'meshcom:map:zoom';
	const STORAGE_MAIDENHEAD = 'meshcom:map:maidenhead';
	const STORAGE_LABELS = 'meshcom:map:labels';
	const STORAGE_CLUSTERING = 'meshcom:map:clustering';
	const STORAGE_RULER = 'meshcom:map:ruler';
	const STORAGE_DM_TRACKING = 'meshcom:map:dm-tracking';

	let {
		positions = [],
		myCall = '',
		events = []
	}: { positions?: MapPosition[]; myCall?: string; events?: StreamEvent[] } = $props();

	let mapElement: HTMLDivElement;
	let tooltipElement: HTMLDivElement;
	let map: any;
	let markerSource: any;
	let rulerSource: any;
	let rulerLayer: any;
	let dmTraceSource: any;
	let dmTraceLayer: any;
	let clusterBubbleLayer: any;
	let maidenheadLayer: any;
	let olContext: any = {};
	let initialized = false;
	let showMaidenhead = $state(false);
	let showLabels = $state(true);
	let showClustering = $state(true);
	let showRuler = $state(false);
	let showDmTracking = $state(false);
	let now = $state(Date.now());
	let tickerHandle: ReturnType<typeof setInterval> | null = null;
	let dmTraceTickerHandle: ReturnType<typeof setInterval> | null = null;

	let visibleCount = $derived(positions.filter((p) => nodeFreshness(p, now) !== 'hidden').length);
	let myCallPosition = $derived(
		myCall !== ''
			? (positions.find((p) => p.source.toUpperCase() === myCall.toUpperCase()) ?? null)
			: null
	);

	$effect(() => {
		positions;
		now;
		showLabels;
		myCall;
		showRuler;
		if (initialized) updateMarkers();
	});

	$effect(() => {
		events;
		positions;
		showDmTracking;
		if (initialized) updateDmTraceLayer(Date.now());
	});

	onMount(async () => {
		const [
			{ Map, View },
			{ Tile: TileLayer, Vector: VectorLayer },
			{ OSM, Vector: VectorSource, Cluster },
			{ fromLonLat, toLonLat },
			{ Style, Fill, Stroke, Circle: CircleStyle, Text },
			Feature,
			{ Point, LineString },
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

		olContext = {
			fromLonLat,
			toLonLat,
			Style,
			Fill,
			Stroke,
			CircleStyle,
			Text,
			Feature,
			Point,
			LineString
		};

		markerSource = new VectorSource();
		rulerSource = new VectorSource();
		rulerLayer = new VectorLayer({ source: rulerSource });
		dmTraceSource = new VectorSource();
		dmTraceLayer = new VectorLayer({ source: dmTraceSource });
		const clusterSource = new Cluster({ source: markerSource, distance: 30 });

		// Only render a bubble when 4+ nodes collapse into one cluster point.
		// For ≤3 nodes the individual markers (markerLayer below) show through.
		function clusterBubbleStyleFn(feature: any) {
			const features = feature.get('features') as any[];
			const count = features?.length ?? 0;
			if (count <= 3) return undefined;
			const radius = Math.round(12 + Math.log2(count) * 2.5);
			const fontSize = radius < 16 ? 11 : 13;
			return new Style({
				image: new CircleStyle({
					radius,
					fill: new Fill({ color: 'rgba(59,130,246,0.9)' }),
					stroke: new Stroke({ color: '#fff', width: 2 })
				}),
				text: new Text({
					text: String(count),
					fill: new Fill({ color: '#fff' }),
					font: `bold ${fontSize}px Inter, sans-serif`
				})
			});
		}

		maidenheadLayer = getMaidenheadLayer();
		maidenheadLayer.setVisible(showMaidenhead);
		clusterBubbleLayer = new VectorLayer({ source: clusterSource, style: clusterBubbleStyleFn });
		clusterBubbleLayer.setVisible(showClustering);

		map = new Map({
			target: mapElement,
			controls: [],
			layers: [
				new TileLayer({ source: new OSM() }),
				maidenheadLayer,
				dmTraceLayer,
				rulerLayer,
				new VectorLayer({ source: markerSource }),
				clusterBubbleLayer
			],
			view: new View({
				center: fromLonLat([16.514880667572868, 50.409595981353704]),
				zoom: 3.4,
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
			const clustered = feature.get('features') as any[] | undefined;
			if (clustered === undefined) {
				const position = feature.get('position') as MapPosition | undefined;
				if (!position) {
					tooltip.setPosition(undefined);
					tooltipElement.classList.add('hidden');
					return;
				}
				tooltipElement.innerHTML = buildMarkerTooltipHtml(position);
			} else if (clustered.length === 0) {
				tooltip.setPosition(undefined);
				tooltipElement.classList.add('hidden');
				return;
			} else if (clustered.length === 1) {
				const position = clustered[0].get('position') as MapPosition;
				tooltipElement.innerHTML = buildMarkerTooltipHtml(position);
			} else {
				const names = clustered
					.map((f: any) => escHtml((f.get('position') as MapPosition)?.source ?? ''))
					.filter(Boolean)
					.join('<br>');
				tooltipElement.innerHTML = `<strong>${clustered.length} stazioni</strong><br>${names}`;
			}
			tooltipElement.classList.remove('hidden');
			tooltip.setPosition(event.coordinate);
		});

		map.on('moveend', saveMapState);

		initialized = true;
		loadMapState();
		updateMarkers();
		updateDmTraceLayer(Date.now());
		tickerHandle = setInterval(() => {
			now = Date.now();
		}, 30_000);
		dmTraceTickerHandle = setInterval(() => {
			if (showDmTracking) updateDmTraceLayer(Date.now());
		}, 1_000);
	});

	onDestroy(() => {
		if (tickerHandle !== null) clearInterval(tickerHandle);
		if (dmTraceTickerHandle !== null) clearInterval(dmTraceTickerHandle);
		map?.setTarget(undefined);
	});

	function loadMapState() {
		const view = map?.getView();
		if (!view || !olContext.fromLonLat) return;

		const centerStr = localStorage.getItem(STORAGE_CENTER);
		const zoomStr = localStorage.getItem(STORAGE_ZOOM);
		const maidenheadStr = localStorage.getItem(STORAGE_MAIDENHEAD);

		if (centerStr && zoomStr) {
			try {
				const [lon, lat] = JSON.parse(centerStr) as [number, number];
				view.setCenter(olContext.fromLonLat([lon, lat]));
				view.setZoom(parseFloat(zoomStr));
			} catch {
				/* ignore malformed storage */
			}
		}

		if (maidenheadStr !== null) {
			showMaidenhead = maidenheadStr === 'true';
			maidenheadLayer?.setVisible(showMaidenhead);
		}

		const labelsStr = localStorage.getItem(STORAGE_LABELS);
		if (labelsStr !== null) {
			showLabels = labelsStr === 'true';
		}

		const clusteringStr = localStorage.getItem(STORAGE_CLUSTERING);
		if (clusteringStr !== null) {
			showClustering = clusteringStr === 'true';
			clusterBubbleLayer?.setVisible(showClustering);
		}

		const rulerStr = localStorage.getItem(STORAGE_RULER);
		if (rulerStr !== null) {
			showRuler = rulerStr === 'true';
		}

		const dmTrackingStr = localStorage.getItem(STORAGE_DM_TRACKING);
		if (dmTrackingStr !== null) {
			showDmTracking = dmTrackingStr === 'true';
		}
	}

	function saveMapState() {
		const view = map?.getView();
		if (!view || !olContext.toLonLat) return;

		const center = view.getCenter();
		if (center) {
			const [lon, lat] = olContext.toLonLat(center) as [number, number];
			localStorage.setItem(STORAGE_CENTER, JSON.stringify([lon, lat]));
		}
		const zoom = view.getZoom();
		if (zoom != null) localStorage.setItem(STORAGE_ZOOM, String(zoom));
		localStorage.setItem(STORAGE_MAIDENHEAD, String(showMaidenhead));
		localStorage.setItem(STORAGE_LABELS, String(showLabels));
		localStorage.setItem(STORAGE_CLUSTERING, String(showClustering));
		localStorage.setItem(STORAGE_RULER, String(showRuler));
		localStorage.setItem(STORAGE_DM_TRACKING, String(showDmTracking));
	}

	function updateMarkers() {
		const { fromLonLat, Feature, Point, LineString, Style, Fill, Stroke, CircleStyle, Text } =
			olContext;
		if (!markerSource || !fromLonLat || !Feature) return;
		markerSource.clear();
		rulerSource?.clear();

		if (showRuler && rulerSource && LineString) {
			for (const link of buildRulerLinks(myCallPosition, positions, now)) {
				const lineFeature = new Feature({
					geometry: new LineString([
						fromLonLat([link.from.lon, link.from.lat]),
						fromLonLat([link.to.lon, link.to.lat])
					])
				});
				lineFeature.setStyle(
					new Style({
						zIndex: 2,
						stroke: new Stroke({
							color: 'rgba(34,197,94,0.92)',
							width: 2
						}),
						text: new Text({
							text: link.label,
							placement: 'line',
							overflow: true,
							font: '600 10px Inter, sans-serif',
							fill: new Fill({ color: '#ecfdf5' }),
							stroke: new Stroke({ color: '#14532d', width: 3 })
						})
					})
				);
				rulerSource.addFeature(lineFeature);
			}
		}

		for (const position of positions) {
			const freshness = nodeFreshness(position, now);
			if (freshness === 'hidden') continue;
			const isMyCall = myCall !== '' && position.source.toUpperCase() === myCall.toUpperCase();
			const feature = new Feature({
				geometry: new Point(fromLonLat([position.lon, position.lat])),
				position
			});
			feature.setStyle(
				new Style({
					zIndex: isMyCall ? MYCALL_ZINDEX : FRESHNESS_ZINDEX[freshness],
					image: new CircleStyle({
						radius: isMyCall ? 8 : 6,
						fill: new Fill({ color: isMyCall ? MYCALL_FILL : FRESHNESS_FILL[freshness] }),
						stroke: new Stroke({ color: '#ecfeff', width: 2 })
					}),
					text: showLabels
						? new Text({
								text: position.source,
								font: '600 11px Inter, sans-serif',
								offsetY: -22,
								fill: new Fill({ color: '#f9fafb' }),
								stroke: new Stroke({ color: '#111827', width: 3 })
							})
						: undefined
				})
			);
			markerSource.addFeature(feature);
		}
	}

	function updateDmTraceLayer(nowMs: number) {
		const { fromLonLat, Feature, LineString, Style, Stroke } = olContext;
		if (!dmTraceSource || !fromLonLat || !Feature || !LineString || !showDmTracking) {
			dmTraceSource?.clear();
			return;
		}
		dmTraceSource.clear();
		const segments = buildRealtimeDmTraceSegments(positions, events, nowMs);
		for (const segment of segments) {
			const feature = new Feature({
				geometry: new LineString([
					fromLonLat([segment.from.lon, segment.from.lat]),
					fromLonLat([segment.to.lon, segment.to.lat])
				])
			});
			feature.setStyle(
				new Style({
					zIndex: 1,
					stroke: new Stroke({
						color: segment.isAck ? 'rgba(168,85,247,0.95)' : 'rgba(56,189,248,0.95)',
						width: 2,
						lineDash: [7, 7]
					})
				})
			);
			dmTraceSource.addFeature(feature);
		}
	}

	function zoomBy(delta: number) {
		const view = map?.getView();
		if (!view) return;
		view.animate({ zoom: (view.getZoom() ?? 6) + delta, duration: 180 });
	}

	function recenter() {
		const view = map?.getView();
		if (!view || !olContext.fromLonLat || !myCallPosition) return;
		view.animate({
			center: olContext.fromLonLat([myCallPosition.lon, myCallPosition.lat]),
			zoom: 10,
			duration: 350
		});
	}

	function toggleClustering() {
		showClustering = !showClustering;
		clusterBubbleLayer?.setVisible(showClustering);
		saveMapState();
	}

	function toggleLabels() {
		showLabels = !showLabels;
		updateMarkers();
		saveMapState();
	}

	function toggleDmTracking() {
		showDmTracking = !showDmTracking;
		updateDmTraceLayer(Date.now());
		saveMapState();
	}

	function toggleRuler() {
		showRuler = !showRuler;
		updateMarkers();
		saveMapState();
	}

	function toggleMaidenhead() {
		showMaidenhead = !showMaidenhead;
		maidenheadLayer?.setVisible(showMaidenhead);
		saveMapState();
	}

	function buildMarkerTooltipHtml(position: MapPosition): string {
		if (myCall !== '' && position.source.toUpperCase() === myCall.toUpperCase()) {
			return buildOwnMarkerTooltipHtml(position);
		}
		return buildTooltipHtml(position);
	}
</script>

<div class="relative h-full w-full overflow-hidden bg-[#0d1017]">
	<div bind:this={mapElement} class="h-full w-full"></div>

	<div class="absolute left-2 top-2 z-[1000] flex flex-col rounded shadow-md">
		<button
			class="flex h-7 w-7 items-center justify-center rounded-t border-b border-gray-300 bg-white text-gray-800 hover:bg-gray-100"
			onclick={() => zoomBy(1)}
		>
			<MdiIcon path={mdiPlus} size={17} />
		</button>
		<button
			class="flex h-7 w-7 items-center justify-center border-b border-gray-300 bg-white text-gray-800 hover:bg-gray-100"
			onclick={() => zoomBy(-1)}
		>
			<MdiIcon path={mdiMinus} size={17} />
		</button>
		<button
			class="flex h-7 w-7 items-center justify-center border-b border-gray-300 bg-white hover:bg-gray-100 {showMaidenhead
				? 'text-gray-800'
				: 'text-gray-400 opacity-70'}"
			title="Toggle Maidenhead grid"
			onclick={toggleMaidenhead}
		>
			<MdiIcon path={showMaidenhead ? mdiGrid : mdiGridOff} size={16} />
		</button>
		<button
			class="flex h-7 w-7 items-center justify-center border-b border-gray-300 bg-white hover:bg-gray-100 {showClustering
				? 'text-gray-800'
				: 'text-gray-400 opacity-70'}"
			title="Toggle clustering"
			onclick={toggleClustering}
		>
			<MdiIcon path={showClustering ? mdiLayersTriple : mdiLayersTripleOutline} size={16} />
		</button>
		<button
			class="flex h-7 w-7 items-center justify-center border-b border-gray-300 bg-white hover:bg-gray-100 {showLabels
				? 'text-gray-800'
				: 'text-gray-400 opacity-70'}"
			title="Toggle callsign labels"
			onclick={toggleLabels}
		>
			<MdiIcon path={showLabels ? mdiTagText : mdiTagOff} size={16} />
		</button>
		<button
			class="flex h-7 w-7 items-center justify-center border-b border-gray-300 bg-white hover:bg-gray-100 {showDmTracking
				? 'text-gray-800'
				: 'text-gray-400 opacity-70'}"
			title="Toggle realtime DM tracking"
			onclick={toggleDmTracking}
		>
			<MdiIcon path={mdiMapMarkerPath} size={16} />
		</button>
		<button
			class="flex h-7 w-7 items-center justify-center rounded-b bg-white hover:bg-gray-100 {showRuler
				? 'text-gray-800'
				: 'text-gray-400 opacity-70'}"
			title="Toggle ruler distances"
			onclick={toggleRuler}
		>
			<MdiIcon path={mdiRuler} size={16} />
		</button>
	</div>

	{#if myCallPosition}
		<button
			class="absolute right-2 top-2 z-[1000] rounded border border-gray-300 bg-white px-2 py-1 text-xs font-semibold text-gray-800 shadow hover:bg-gray-100"
			title="Center on {myCall}"
			onclick={recenter}
		>
			<span class="flex items-center gap-1"
				><MdiIcon path={mdiCrosshairsGps} size={14} /> {myCall}</span
			>
		</button>
	{/if}

	<div
		class="absolute bottom-2 left-2 z-[1000] rounded border border-gray-700 bg-[#1e2330]/90 px-2 py-1 font-mono text-[11px] text-gray-200"
	>
		{visibleCount} positions · OSM · Maidenhead
	</div>

	<div
		class="absolute bottom-2 right-2 z-[1000] rounded border border-gray-700 bg-[#1e2330]/90 px-2 py-1 text-[10px] text-gray-300"
	>
		© OpenStreetMap contributors
	</div>

	<div
		bind:this={tooltipElement}
		class="pointer-events-none absolute z-[2000] hidden min-w-[160px] whitespace-nowrap rounded border border-gray-700 bg-gray-950 px-3 py-2 text-[11px] leading-5 text-white shadow-md"
	></div>
</div>
