<script lang="ts">
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { packetFromEvent, splitSourcePath, eventSummary } from '$lib/api/events';
	import { mdiForEvent, packetTone, iconTooltip, messageText, isMessageEvent } from '$lib/ui/stream';
	import { formatDate, formatTime } from '$lib/ui/format';
	import { eventsState } from '$lib/stores/events.svelte';
	import type { StreamEvent } from '$lib/api/types';

	let { events }: { events: StreamEvent[] } = $props();

	const recent = $derived(events.slice(0, 5));

	function sender(event: StreamEvent): string {
		return splitSourcePath(packetFromEvent(event)?.src).origin;
	}

	function focus(event: StreamEvent) {
		const packet = packetFromEvent(event);
		const origin = splitSourcePath(packet?.src).origin;
		if (!origin) return;

		let lat = packet?.lat;
		let lon = packet?.long;

		if (lat == null || lon == null) {
			const pos = eventsState.mapPositions.find(
				(p) => p.source.toUpperCase() === origin.toUpperCase()
			);
			if (!pos) return;
			lat = pos.lat;
			lon = pos.lon;
		}

		eventsState.focusOnNode(origin, lat, lon);
	}
</script>

{#if recent.length > 0}
	<div
		class="hidden md:flex w-96 flex-col gap-0.5 rounded-2xl border border-ink-dim/20 bg-base/80 px-2 py-1.5 text-[11px] shadow-md backdrop-blur-sm"
	>
		<span class="mb-0.5 text-[10px] font-semibold uppercase tracking-wider text-ink-muted"
			>Live stream</span
		>
		{#each recent as event (event.id)}
			<button
				type="button"
				class="flex min-w-0 items-center gap-1.5 rounded-lg px-1 py-0.5 text-left hover:bg-surface-hi/50"
				onclick={() => focus(event)}
			>
				<span class="shrink-0 font-mono text-[10px] text-ink">
					<span class="text-ink-muted">{formatDate(event.receivedAt)}</span>
					{formatTime(event.receivedAt)}
				</span>
				<span
					class="flex h-5 w-5 shrink-0 items-center justify-center rounded border {packetTone(event)}"
					title={iconTooltip(event)}
				>
					<MdiIcon path={mdiForEvent(event)} size={14} />
				</span>
				<span class="min-w-0 truncate">
					<span class="font-bold text-ink">{sender(event) || eventSummary(event)}</span>
					{#if isMessageEvent(event)}
						<span class="text-ink-dim"> · </span><span class="italic text-ink-muted">"{messageText(event)}"</span>
					{/if}
				</span>
			</button>
		{/each}
	</div>
{/if}
