<script lang="ts">
	import type { Snippet } from 'svelte';
	import logo from '$lib/assets/gomeshcom-logo.png';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import ConnectionOverlay from '$lib/components/ConnectionOverlay.svelte';
	import MobileDrawer from '$lib/components/MobileDrawer.svelte';
	import TopNav from '$lib/components/TopNav.svelte';
	import TxDisabledBanner from '$lib/components/TxDisabledBanner.svelte';
	import { eventJSON, packetBadge } from '$lib/api/events';
	import { formatTime } from '$lib/ui/format';
	import { chatMdiIcon } from '$lib/ui/chat-records';
	import { mdiForEvent, packetTone } from '$lib/ui/stream';
	import { chatState } from '$lib/stores/chat.svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { eventsState } from '$lib/stores/events.svelte';
	import { stationStore } from '$lib/stores/station';
	import { viewState } from '$lib/stores/view.svelte';
	import type { AppController } from '$lib/app/app-controller.svelte';
	import { mdiClose } from '@mdi/js';
	import NodeCombobox from '$lib/components/NodeCombobox.svelte';
	import ChannelCombobox from '$lib/components/ChannelCombobox.svelte';
	import ChannelShowModal from '$lib/components/chat/ChannelShowModal.svelte';

	let { app, children }: { app: AppController; children: Snippet } = $props();
</script>

{#if $stationStore?.txDisabled}
	<TxDisabledBanner />
{/if}

<main class="relative flex h-screen min-h-0 flex-col bg-[#111827] text-gray-100">
	<ConnectionOverlay state={connectionState.state} />

	{#if app.authModalOpen}
		<div
			class="fixed inset-0 z-[10000] flex items-center justify-center bg-[#020617]/75 p-4 backdrop-blur-sm"
		>
			<form
				class="w-full max-w-sm rounded-2xl border border-slate-700 bg-slate-950 p-5 shadow-2xl"
				onsubmit={(event) => {
					event.preventDefault();
					void app.submitAuth();
				}}
			>
				<div class="mb-4">
					<h2 class="text-lg font-semibold text-slate-100">Sign in</h2>
					<p class="mt-1 text-sm text-slate-400">Protected MeshCom session on local server.</p>
				</div>
				<div class="space-y-3">
					<label class="block text-sm text-slate-300">
						<span class="mb-1 block">Username</span>
						<input
							class="w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-slate-100 outline-none placeholder:text-slate-500 focus:border-amber-300"
							type="text"
							autocomplete="username"
							bind:value={app.authUsername}
						/>
					</label>
					<label class="block text-sm text-slate-300">
						<span class="mb-1 block">Password</span>
						<input
							class="w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-slate-100 outline-none placeholder:text-slate-500 focus:border-amber-300"
							type="password"
							autocomplete="current-password"
							bind:value={app.authPassword}
						/>
					</label>
				</div>
				{#if app.authError}
					<p
						class="mt-3 rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-200"
					>
						{app.authError}
					</p>
				{/if}
				<button
					class="mt-4 w-full rounded-lg bg-amber-300 px-3 py-2 text-sm font-semibold text-slate-950 transition hover:bg-amber-200 disabled:cursor-not-allowed disabled:opacity-60"
					type="submit"
					disabled={app.authSubmitting}
				>
					{app.authSubmitting ? 'Signing in...' : 'Sign in'}
				</button>
			</form>
		</div>
	{/if}

	<header
		class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 bg-[#2d3345] px-4"
	>
		<div class="flex items-center gap-3">
			<div class="flex items-center gap-2">
				<img src={logo} alt="" class="h-7 w-7 rounded-md" />
				<span class="font-mono text-sm font-bold tracking-wide text-blue-300">goMeshCom</span>
				<span
					data-testid="status-dot"
					class="md:hidden h-2 w-2 rounded-full {app.statusClass[connectionState.state]}"
				></span>
			</div>
			<div class="hidden md:block h-4 w-px bg-gray-600"></div>
			<div data-testid="status-pill" class="hidden md:flex items-center gap-2 text-xs">
				<span class="h-2.5 w-2.5 rounded-full {app.statusClass[connectionState.state]}"></span>
				<span class="font-medium text-gray-300">{app.statusText[connectionState.state]}</span>
			</div>
		</div>

		<div class="hidden md:flex items-center">
			<TopNav />
		</div>

		<div class="flex items-center gap-3">
			<div
				class="rounded border border-blue-500/40 bg-blue-500/10 px-2 py-1 font-mono text-xs font-semibold text-blue-200"
			>
				{connectionState.stationCallsign || 'NO-CALL'}
			</div>
			<div data-testid="packet-counter" class="hidden md:block font-mono text-xs text-gray-400">
				{eventsState.events.length} packets
			</div>
			<button
				type="button"
				class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-gray-500 hover:text-gray-200 md:hidden"
				aria-label="Open menu"
				onclick={() => viewState.openDrawer()}
			>
				<svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
					<path d="M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z" />
				</svg>
			</button>
		</div>
	</header>

	<div class="flex min-h-0 flex-1 overflow-hidden">
		<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
			{@render children()}
		</div>
	</div>

	<MobileDrawer />
</main>

{#if eventsState.rawEvent}
	<div
		class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
	>
		<div
			class="flex max-h-[86vh] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 px-4">
				<div class="flex items-center gap-2">
					<span
						class="flex h-7 w-7 items-center justify-center rounded border {packetTone(
							eventsState.rawEvent
						)}"
					>
						<MdiIcon path={mdiForEvent(eventsState.rawEvent)} size={16} />
					</span>
					<div>
						<div class="text-sm font-semibold text-gray-100">Formatted JSON</div>
						<div class="font-mono text-[11px] text-gray-500">
							{packetBadge(eventsState.rawEvent)} - {formatTime(eventsState.rawEvent.receivedAt)}
						</div>
					</div>
				</div>
				<button
					type="button"
					class="flex h-8 w-8 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (eventsState.rawEvent = null)}
				>
					<MdiIcon path={mdiClose} size={18} />
				</button>
			</div>
			<pre
				class="min-h-0 overflow-auto p-4 font-mono text-xs leading-relaxed text-gray-200">{eventJSON(
					eventsState.rawEvent
				)}</pre>
		</div>
	</div>
{/if}

{#if chatState.rawChatRecord}
	<div
		class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
	>
		<div
			class="flex max-h-[86vh] w-full max-w-4xl flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-12 shrink-0 items-center justify-between border-b border-gray-700/60 px-4">
				<div class="flex items-center gap-2">
					<span
						class="flex h-7 w-7 items-center justify-center rounded border border-blue-500/70 bg-blue-500/25 text-blue-200"
					>
						<MdiIcon path={chatMdiIcon(chatState.rawChatRecord)} size={16} />
					</span>
					<div>
						<div class="text-sm font-semibold text-gray-100">Chat Record JSON</div>
						<div class="font-mono text-[11px] text-gray-500">
							msg - {formatTime(chatState.rawChatRecord.received_at)}
						</div>
					</div>
				</div>
				<button
					type="button"
					class="flex h-8 w-8 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (chatState.rawChatRecord = null)}
				>
					<MdiIcon path={mdiClose} size={18} />
				</button>
			</div>
			<pre
				class="min-h-0 overflow-auto p-4 font-mono text-xs leading-relaxed text-gray-200">{JSON.stringify(
					chatState.rawChatRecord,
					null,
					2
				)}</pre>
		</div>
	</div>
{/if}

{#if chatState.deleteConfirmOpen}
	<button
		type="button"
		class="fixed inset-0 z-[9998] bg-black/70 backdrop-blur-sm"
		aria-label="Close delete confirmation"
		onclick={() => (chatState.deleteConfirmOpen = false)}
	></button>
	<div class="pointer-events-none fixed inset-0 z-[9999] flex items-center justify-center p-4">
		<div
			class="pointer-events-auto w-full max-w-sm rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-11 items-center justify-between border-b border-gray-700/60 px-4">
				<span class="text-sm font-semibold text-gray-100">
					{chatState.isBroadcastTarget ? 'Clear messages' : 'Delete conversation'}
				</span>
				<button
					type="button"
					class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (chatState.deleteConfirmOpen = false)}
				>
					<MdiIcon path={mdiClose} size={16} />
				</button>
			</div>
			<div class="p-4">
				<p class="text-sm text-gray-300">
					{#if chatState.isBroadcastTarget}
						Clear all messages in <span class="font-semibold text-white">Broadcast</span>?
					{:else}
						Delete <span class="font-semibold text-white">{chatState.chatTarget.value}</span>?
					{/if}
					This removes {chatState.displayChatRecords.length} message(s) from disk.
				</p>
				{#if chatState.deleteError}
					<p class="mt-2 text-xs text-red-400">{chatState.deleteError}</p>
				{/if}
				<div class="mt-4 flex justify-end gap-2">
					<button
						type="button"
						class="rounded border border-gray-700/60 px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200"
						onclick={() => (chatState.deleteConfirmOpen = false)}
						disabled={chatState.deleting}>Cancel</button
					>
					<button
						type="button"
						class="rounded border border-red-500/40 bg-red-600/80 px-3 py-1.5 text-xs text-white hover:bg-red-500 disabled:opacity-50"
						onclick={() => void app.confirmDelete()}
						disabled={chatState.deleting}
					>
						{chatState.deleting ? '...' : chatState.isBroadcastTarget ? 'Clear' : 'Delete'}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

{#if chatState.newDmOpen}
	<button
		type="button"
		class="fixed inset-0 z-[9998] bg-black/70 backdrop-blur-sm"
		aria-label="Close new direct message"
		onclick={() => (chatState.newDmOpen = false)}
	></button>
	<div class="pointer-events-none fixed inset-0 z-[9999] flex items-center justify-center p-4">
		<div
			class="pointer-events-auto w-full max-w-sm rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-11 items-center justify-between border-b border-gray-700/60 px-4">
				<span class="text-sm font-semibold text-gray-100">New Direct Message</span>
				<button
					type="button"
					class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (chatState.newDmOpen = false)}
				>
					<MdiIcon path={mdiClose} size={16} />
				</button>
			</div>
			<div class="p-4">
				<label class="mb-1.5 block text-xs text-gray-400" for="new-dm-callsign">Callsign</label>
				<NodeCombobox
					nodes={eventsState.mapPositions}
					bind:value={chatState.newDmCallsign}
					onconfirm={() => app.confirmNewDm()}
					onclose={() => (chatState.newDmOpen = false)}
					onValueChange={() => (chatState.newDmError = '')}
				/>
				{#if chatState.newDmError}
					<p class="mt-1 text-xs text-red-400">{chatState.newDmError}</p>
				{/if}
				<div class="mt-3 flex justify-end gap-2">
					<button
						type="button"
						class="rounded border border-gray-700/60 px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200"
						onclick={() => (chatState.newDmOpen = false)}
					>
						Cancel
					</button>
					<button
						type="button"
						class="rounded border border-blue-500/40 bg-blue-600/80 px-3 py-1.5 text-xs text-white hover:bg-blue-500"
						onclick={() => app.confirmNewDm()}
					>
						Open Chat
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

{#if chatState.channelShowOpen}
	<ChannelShowModal onConfirm={() => void app.confirmChannelShow()} />
{/if}

{#if chatState.newChannelOpen}
	<button
		type="button"
		class="fixed inset-0 z-[9998] bg-black/70 backdrop-blur-sm"
		aria-label="Close new channel"
		onclick={() => (chatState.newChannelOpen = false)}
	></button>
	<div class="pointer-events-none fixed inset-0 z-[9999] flex items-center justify-center p-4">
		<div
			class="pointer-events-auto w-full max-w-sm rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		>
			<div class="flex h-11 items-center justify-between border-b border-gray-700/60 px-4">
				<span class="text-sm font-semibold text-gray-100">Join Channel</span>
				<button
					type="button"
					class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
					aria-label="Close"
					onclick={() => (chatState.newChannelOpen = false)}
				>
					<MdiIcon path={mdiClose} size={16} />
				</button>
			</div>
			<div class="p-4">
				<label class="mb-1.5 block text-xs text-gray-400" for="new-channel-value">
					Channel — <span class="font-mono">*</span> for broadcast, numeric for groups
				</label>
				<ChannelCombobox
					bind:value={chatState.newChannelValue}
					onconfirm={() => void app.confirmNewChannel()}
					onclose={() => (chatState.newChannelOpen = false)}
					onValueChange={() => (chatState.newChannelError = '')}
				/>
				{#if chatState.newChannelError}
					<p class="mt-1 text-xs text-red-400">{chatState.newChannelError}</p>
				{/if}
				<div class="mt-3 flex justify-end gap-2">
					<button
						type="button"
						class="rounded border border-gray-700/60 px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200"
						onclick={() => (chatState.newChannelOpen = false)}
					>
						Cancel
					</button>
					<button
						type="button"
						class="rounded border border-blue-500/40 bg-blue-600/80 px-3 py-1.5 text-xs text-white hover:bg-blue-500"
						onclick={() => void app.confirmNewChannel()}
					>
						Join
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
