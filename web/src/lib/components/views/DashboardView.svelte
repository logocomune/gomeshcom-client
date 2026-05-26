<script lang="ts">
	import { goto } from '$app/navigation';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { cleanMessage, messageKind, packetBadge, packetFromEvent, splitSourcePath } from '$lib/api/events';
	import { mdiForEvent } from '$lib/ui/stream';
	import { chatState } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
	import {
		mdiArrowRight,
		mdiAccessPoint,
		mdiAccessPointNetwork,
		mdiEmailMultipleOutline,
		mdiBroadcast,
		mdiMessageTextOutline,
		mdiPulse
	} from '@mdi/js';
	import type { ChatRecord } from '$lib/api/types';

	const STATE_COLORS: Record<string, string> = {
		connected: 'bg-mint',
		connecting: 'bg-warm',
		disconnected: 'bg-coral',
		unauthenticated: 'bg-blaze'
	};

	const STATE_TEXT: Record<string, string> = {
		connected: 'text-mint',
		connecting: 'text-warm',
		disconnected: 'text-coral',
		unauthenticated: 'text-blaze'
	};

	const STATE_LABELS: Record<string, string> = {
		connected: 'Connected',
		connecting: 'Connecting…',
		disconnected: 'Disconnected',
		unauthenticated: 'Auth required'
	};

	const KIND_COLORS: Record<string, { circle: string; iconClass: string; pill: string }> = {
		msg: { circle: 'bg-mint/15', iconClass: 'text-mint', pill: 'bg-mint/20 text-mint' },
		pos: { circle: 'bg-azure/15', iconClass: 'text-azure', pill: 'bg-azure/20 text-azure' },
		tele: { circle: 'bg-lavender/15', iconClass: 'text-lavender', pill: 'bg-lavender/20 text-lavender' },
		error: { circle: 'bg-coral/15', iconClass: 'text-coral', pill: 'bg-coral/20 text-coral' }
	};

	let stateDot = $derived(STATE_COLORS[connectionState.state] ?? 'bg-ink-dim');
	let stateText = $derived(STATE_TEXT[connectionState.state] ?? 'text-ink-dim');
	let stateLabel = $derived(STATE_LABELS[connectionState.state] ?? connectionState.state);

	let recentMessages = $derived.by(() => {
		const all: ChatRecord[] = [];
		for (const records of Object.values(chatState.chatHistory)) {
			for (const r of records) {
				const kind = messageKind(r.msg).kind;
				if (kind === 'ack' || kind === 'reject' || kind === 'time' || kind === 'config') continue;
				all.push(r);
			}
		}
		return all.sort((a, b) => b.received_at.localeCompare(a.received_at)).slice(0, 4);
	});

	function kindColor(badge: string) {
		return KIND_COLORS[badge] ?? { circle: 'bg-warm/15', iconClass: 'text-warm', pill: 'bg-warm/20 text-warm' };
	}

	function timeAgo(isoTs: string): string {
		const diff = Date.now() - new Date(isoTs).getTime();
		if (isNaN(diff)) return '—';
		const secs = Math.floor(diff / 1000);
		if (secs < 60) return `${secs}s`;
		const mins = Math.floor(secs / 60);
		if (mins < 60) return `${mins}m`;
		const hours = Math.floor(mins / 60);
		if (hours < 24) return `${hours}h`;
		return `${Math.floor(hours / 24)}d`;
	}

	function truncateMsg(msg: string, max = 55): string {
		const clean = cleanMessage(msg);
		return clean.length > max ? `${clean.slice(0, max)}…` : clean;
	}

	function originFrom(src: string | undefined): string {
		if (!src) return '?';
		return splitSourcePath(src).origin;
	}
</script>

<div class="min-h-0 flex-1 overflow-y-auto p-4 md:p-6">
	<div class="mx-auto max-w-4xl space-y-5">

		<!-- Connection status -->
		<div
			data-testid="dashboard-status"
			class="rounded-2xl border border-warm/20 bg-surface/70 px-4 py-3.5 backdrop-blur-sm"
		>
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2.5">
					<span class="relative flex h-3 w-3 shrink-0">
						{#if connectionState.state === 'connected'}
							<span class="absolute inline-flex h-full w-full animate-ping rounded-full {stateDot} opacity-50"></span>
						{/if}
						<span class="relative inline-flex h-3 w-3 rounded-full {stateDot}"></span>
					</span>
					<span class="text-sm font-semibold {stateText}">{stateLabel}</span>
					{#if connectionState.stationCallsign}
						<span class="flex items-center gap-1.5">
							<span class="text-warm"><MdiIcon path={mdiAccessPoint} size={14} /></span>
							<span class="font-mono text-sm tracking-wider text-warm">{connectionState.stationCallsign}</span>
						</span>
					{/if}
				</div>
				<div class="flex items-center gap-2">
					{#if connectionState.txDisabled}
						<span class="rounded-full bg-blaze/20 px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-blaze">
							TX off
						</span>
					{/if}
					{#if connectionState.appVersion}
						<span class="rounded-full bg-surface-soft px-2.5 py-0.5 font-mono text-[11px] text-ink-dim">
							{connectionState.appVersion}
						</span>
					{/if}
				</div>
			</div>
		</div>

		<!-- Metric cards -->
		<div class="grid grid-cols-3 gap-4">

			<!-- Nodes -->
			<button
				type="button"
				data-testid="dashboard-nodes-card"
				onclick={() => goto('/nodes')}
				class="group relative overflow-hidden rounded-2xl border border-warm/20 bg-gradient-to-br from-surface to-surface-soft p-4 text-left transition-all hover:-translate-y-0.5 hover:border-warm/60 hover:shadow-[0_8px_30px_rgb(251,191,36,0.12)]"
			>
				<div class="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-warm/10 blur-2xl"></div>
				<div class="flex items-start justify-between">
					<div class="rounded-xl bg-warm/15 p-2">
						<span class="text-warm"><MdiIcon path={mdiAccessPointNetwork} size={24} /></span>
					</div>
				</div>
				<div class="mt-3 font-mono text-4xl font-bold text-ink">{eventsState.mapPositions.length}</div>
				<div class="mt-1 text-xs uppercase tracking-widest text-ink-muted">Nodes</div>
				<div class="absolute bottom-3 right-3 opacity-0 transition-opacity group-hover:opacity-100">
					<span class="text-warm"><MdiIcon path={mdiArrowRight} size={14} /></span>
				</div>
			</button>

			<!-- Unread -->
			<button
				type="button"
				data-testid="dashboard-unread-card"
				onclick={() => goto('/chat')}
				class="group relative overflow-hidden rounded-2xl border border-warm/20 bg-gradient-to-br from-surface to-surface-soft p-4 text-left transition-all hover:-translate-y-0.5 hover:border-azure/60 hover:shadow-[0_8px_30px_rgb(96,165,250,0.12)]"
			>
				<div class="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-azure/10 blur-2xl"></div>
				<div class="flex items-start justify-between">
					<div class="rounded-xl bg-azure/15 p-2">
						<span class="text-azure"><MdiIcon path={mdiEmailMultipleOutline} size={24} /></span>
					</div>
					{#if chatState.visibleUnreadIds.size > 0}
						<span class="rounded-full bg-coral px-2 py-0.5 text-[10px] font-bold text-white">
							{chatState.visibleUnreadIds.size}
						</span>
					{/if}
				</div>
				<div class="mt-3 font-mono text-4xl font-bold {chatState.visibleUnreadIds.size > 0 ? 'text-azure' : 'text-ink'}">
					{chatState.visibleUnreadIds.size}
				</div>
				<div class="mt-1 text-xs uppercase tracking-widest text-ink-muted">Unread</div>
				<div class="absolute bottom-3 right-3 opacity-0 transition-opacity group-hover:opacity-100">
					<span class="text-azure"><MdiIcon path={mdiArrowRight} size={14} /></span>
				</div>
			</button>

			<!-- Events -->
			<button
				type="button"
				data-testid="dashboard-events-card"
				onclick={() => goto('/traffic')}
				class="group relative overflow-hidden rounded-2xl border border-warm/20 bg-gradient-to-br from-surface to-surface-soft p-4 text-left transition-all hover:-translate-y-0.5 hover:border-mint/60 hover:shadow-[0_8px_30px_rgb(52,211,153,0.12)]"
			>
				<div class="absolute -right-6 -top-6 h-20 w-20 rounded-full bg-mint/10 blur-2xl"></div>
				<div class="rounded-xl bg-mint/15 p-2 w-fit">
					<span class="text-mint"><MdiIcon path={mdiBroadcast} size={24} /></span>
				</div>
				<div class="mt-3 font-mono text-4xl font-bold text-ink">{eventsState.events.length}</div>
				<div class="mt-1 text-xs uppercase tracking-widest text-ink-muted">Events</div>
				<div class="absolute bottom-3 right-3 opacity-0 transition-opacity group-hover:opacity-100">
					<span class="text-mint"><MdiIcon path={mdiArrowRight} size={14} /></span>
				</div>
			</button>

		</div>

		<!-- Recent Messages -->
		<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
			<span class="text-ink-dim"><MdiIcon path={mdiMessageTextOutline} size={14} /></span>
			<span>Recent Messages</span>
			<span class="h-px flex-1 bg-ink-dim/20"></span>
			<button
				type="button"
				onclick={() => goto('/chat')}
				class="flex items-center gap-0.5 text-warm transition-colors hover:text-blaze"
			>
				Chat <span><MdiIcon path={mdiArrowRight} size={12} /></span>
			</button>
		</div>

		<div
			data-testid="dashboard-messages"
			class="-mt-2 overflow-hidden rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm"
		>
			{#if recentMessages.length === 0}
				<div class="px-4 py-8 text-center text-xs text-ink-dim">No messages yet</div>
			{:else}
				<div class="divide-y divide-ink-dim/15">
					{#each recentMessages as rec (rec.msg_id ?? rec.received_at + (rec.src ?? ''))}
						<div class="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-hi">
							<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-warm/30 to-blaze/30 font-mono text-xs font-bold text-warm">
								{originFrom(rec.src).slice(0, 2).toUpperCase()}
							</div>
							<div class="min-w-0 flex-1">
								<div class="text-sm font-medium text-ink">{originFrom(rec.src)}</div>
								<div class="truncate text-xs text-ink-muted">{truncateMsg(rec.msg)}</div>
							</div>
							<span class="shrink-0 font-mono text-[10px] text-ink-dim">{timeAgo(rec.received_at)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Recent Traffic -->
		<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
			<span class="text-ink-dim"><MdiIcon path={mdiPulse} size={14} /></span>
			<span>Recent Traffic</span>
			<span class="h-px flex-1 bg-ink-dim/20"></span>
			<button
				type="button"
				onclick={() => goto('/traffic')}
				class="flex items-center gap-0.5 text-warm transition-colors hover:text-blaze"
			>
				Traffic <span><MdiIcon path={mdiArrowRight} size={12} /></span>
			</button>
		</div>

		<div
			data-testid="dashboard-traffic"
			class="-mt-2 overflow-hidden rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm"
		>
			{#if eventsState.events.length === 0}
				<div class="px-4 py-8 text-center text-xs text-ink-dim">
					No events yet — waiting for traffic…
				</div>
			{:else}
				<div class="divide-y divide-ink-dim/15">
					{#each eventsState.events.slice(0, 5) as event (event.id)}
						{@const badge = packetBadge(event)}
						{@const packet = packetFromEvent(event)}
						{@const kc = kindColor(badge)}
						<div class="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-hi">
							<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg {kc.circle}">
								<span class={kc.iconClass}><MdiIcon path={mdiForEvent(event)} size={16} /></span>
							</div>
							<div class="min-w-0 flex-1">
								<span class="font-mono text-sm text-ink">
									{packet ? originFrom(packet.src) : '—'}
								</span>
							</div>
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {kc.pill}">
								{badge}
							</span>
							<span class="shrink-0 font-mono text-[10px] text-ink-dim">{timeAgo(event.receivedAt)}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>

	</div>
</div>
