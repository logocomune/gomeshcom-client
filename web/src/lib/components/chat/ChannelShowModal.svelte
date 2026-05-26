<script lang="ts">
	import { mdiClose, mdiPlus } from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { KNOWN_GROUPS, resolveGroup } from '$lib/api/groups';
	import { isValidChannelShowChannel } from '$lib/api/channelShow';

	interface Props {
		onConfirm: () => void;
	}

	let { onConfirm }: Props = $props();

	type ListEntry = { id: string; flag: string; name: string; id_display: string };

	let searchQuery = $state('');

	const BROADCAST_ENTRY: ListEntry = {
		id: '*',
		flag: '📡',
		name: 'Broadcast',
		id_display: '*'
	};

	const catalogEntries: ListEntry[] = [
		BROADCAST_ENTRY,
		...KNOWN_GROUPS.map((g) => ({
			id: g.group,
			flag: g.flag,
			name: g.note,
			id_display: g.group
		}))
	];

	const unknownEntries = $derived((): ListEntry[] => {
		const catalogIds = new Set(catalogEntries.map((e) => e.id));
		return chatState.conversations
			.filter((c) => c.kind !== 'dm' && c.id !== 'P_broadcast' && !catalogIds.has(c.label))
			.map((c) => ({
				id: c.label,
				flag: resolveGroup(c.label)?.flag ?? '📻',
				name: c.label,
				id_display: c.label
			}));
	});

	const allEntries = $derived((): ListEntry[] => [...catalogEntries, ...unknownEntries()]);

	const filteredEntries = $derived((): ListEntry[] => {
		const q = searchQuery.trim().toLowerCase();
		if (!q) return allEntries();
		return allEntries().filter(
			(e) =>
				e.id_display.toLowerCase().includes(q) ||
				e.name.toLowerCase().includes(q) ||
				e.flag.includes(q)
		);
	});

	// Entries with full label for the selected chips display
	const selectedEntries = $derived((): ListEntry[] => {
		const byId = new Map(allEntries().map((e) => [e.id, e]));
		return chatState.channelShowDraftChannels.map(
			(id) => byId.get(id) ?? { id, flag: '📻', name: id, id_display: id }
		);
	});

	function isSelected(id: string): boolean {
		return chatState.channelShowDraftChannels.includes(id);
	}

	function toggleEntry(id: string) {
		if (chatState.channelShowDraftMode !== 'allowlist') return;
		if (isSelected(id)) {
			chatState.channelShowDraftChannels = chatState.channelShowDraftChannels.filter((c) => c !== id);
		} else {
			chatState.channelShowDraftChannels = [...chatState.channelShowDraftChannels, id];
		}
	}

	function removeSelected(id: string) {
		chatState.channelShowDraftChannels = chatState.channelShowDraftChannels.filter((c) => c !== id);
	}

	function handleAddInput() {
		const val = chatState.channelShowDraftInput.trim();
		if (!isValidChannelShowChannel(val)) {
			chatState.channelShowError = 'Enter * or a numeric channel ID (e.g. 222)';
			return;
		}
		if (!chatState.channelShowDraftChannels.includes(val)) {
			chatState.channelShowDraftChannels = [...chatState.channelShowDraftChannels, val];
		}
		chatState.channelShowDraftInput = '';
		chatState.channelShowError = '';
	}

	function handleInputKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			handleAddInput();
		}
	}

	const isAllowlist = $derived(chatState.channelShowDraftMode === 'allowlist');
</script>

<button
	type="button"
	class="fixed inset-0 z-[9998] bg-black/70 backdrop-blur-sm"
	aria-label="Close channel visibility settings"
	onclick={() => (chatState.channelShowOpen = false)}
></button>

<div class="pointer-events-none fixed inset-0 z-[9999] flex items-center justify-center p-4">
	<div
		class="pointer-events-auto flex w-full max-w-lg flex-col overflow-hidden rounded-md border border-gray-700/60 bg-[#212735] shadow-2xl"
		style="max-height: 82vh"
	>
		<!-- Header -->
		<div class="flex h-11 shrink-0 items-center justify-between border-b border-gray-700/60 px-4">
			<span class="text-sm font-semibold text-gray-100">Visible channels</span>
			<button
				type="button"
				class="flex h-7 w-7 items-center justify-center rounded border border-gray-700/60 text-gray-400 hover:border-red-500/50 hover:text-red-400"
				aria-label="Close"
				onclick={() => (chatState.channelShowOpen = false)}
			>
				<MdiIcon path={mdiClose} size={16} />
			</button>
		</div>

		<div class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-4">
			<!-- Mode toggle -->
			<div class="flex gap-2">
				<button
					type="button"
					class="rounded border px-3 py-1.5 text-xs font-semibold transition-colors
						{!isAllowlist
						? 'border-blue-500/60 bg-blue-600/80 text-white'
						: 'border-gray-700/60 text-gray-400 hover:text-gray-200'}"
					onclick={() => (chatState.channelShowDraftMode = 'all')}
				>
					Show all
				</button>
				<button
					type="button"
					class="rounded border px-3 py-1.5 text-xs font-semibold transition-colors
						{isAllowlist
						? 'border-blue-500/60 bg-blue-600/80 text-white'
						: 'border-gray-700/60 text-gray-400 hover:text-gray-200'}"
					onclick={() => (chatState.channelShowDraftMode = 'allowlist')}
				>
					Allowlist
				</button>
			</div>

			{#if !isAllowlist}
				<p class="text-xs text-gray-500">All channels visible. Switch to Allowlist to pick which to show.</p>
			{:else}
				<!-- Selected chips -->
				{#if selectedEntries().length > 0}
					<div class="flex flex-wrap gap-1.5">
						{#each selectedEntries() as entry (entry.id)}
							<span
								class="flex items-center gap-1 rounded-full border border-blue-500/60 bg-blue-600/40 pl-2 pr-1 py-0.5 text-xs text-white"
							>
								<span class="leading-none">{entry.flag}</span>
								<span class="max-w-[120px] truncate font-medium">{entry.name}</span>
								<span class="font-mono text-blue-300">·{entry.id_display}</span>
								<button
									type="button"
									class="ml-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full text-blue-300 hover:text-white"
									aria-label="Remove {entry.name}"
									onclick={() => removeSelected(entry.id)}
								>
									<MdiIcon path={mdiClose} size={10} />
								</button>
							</span>
						{/each}
					</div>
				{:else}
					<p class="text-xs text-gray-500">No channels selected — all hidden. Pick from list below.</p>
				{/if}

				<!-- Search -->
				<input
					type="search"
					class="w-full rounded border border-gray-700/60 bg-[#111827] px-3 py-1.5 text-xs text-gray-200 outline-none placeholder:text-gray-600 focus:border-blue-500/60"
					placeholder="Search by name, ID or flag…"
					bind:value={searchQuery}
				/>

				<!-- Scrollable list -->
				<div class="min-h-0 overflow-y-auto rounded border border-gray-700/40" style="max-height: 260px">
					{#each filteredEntries() as entry (entry.id)}
						{@const selected = isSelected(entry.id)}
						<button
							type="button"
							class="flex w-full items-center gap-3 border-b border-gray-700/30 px-3 py-2 text-left transition-colors last:border-b-0
								{selected
								? 'bg-blue-600/20 text-white'
								: 'text-gray-300 hover:bg-gray-700/30'}"
							onclick={() => toggleEntry(entry.id)}
						>
							<!-- Checkbox indicator -->
							<span
								class="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded border
									{selected ? 'border-blue-500 bg-blue-500' : 'border-gray-600'}"
							>
								{#if selected}
									<svg viewBox="0 0 10 8" class="h-2 w-2 fill-white">
										<path d="M1 4l3 3 5-6" stroke="white" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
									</svg>
								{/if}
							</span>
							<span class="text-base leading-none">{entry.flag}</span>
							<span class="flex-1 truncate text-xs">{entry.name}</span>
							<span class="shrink-0 font-mono text-[11px] text-gray-500">{entry.id_display}</span>
						</button>
					{/each}
					{#if filteredEntries().length === 0}
						<p class="px-3 py-3 text-xs text-gray-600">No channels match "{searchQuery}"</p>
					{/if}
				</div>

				<!-- Free-text add for unlisted IDs -->
				<div class="flex gap-2">
					<input
						type="text"
						class="min-w-0 flex-1 rounded border border-gray-700/60 bg-[#111827] px-3 py-1.5 font-mono text-xs text-gray-200 outline-none placeholder:text-gray-600 focus:border-blue-500/60"
						placeholder="Custom ID: * or 22299"
						bind:value={chatState.channelShowDraftInput}
						onkeydown={handleInputKeydown}
						oninput={() => (chatState.channelShowError = '')}
					/>
					<button
						type="button"
						class="flex items-center gap-1 rounded border border-gray-700/60 px-2 py-1.5 text-xs text-gray-400 hover:border-gray-500 hover:text-gray-200"
						onclick={handleAddInput}
					>
						<MdiIcon path={mdiPlus} size={13} />
						Add
					</button>
				</div>

				{#if chatState.channelShowError}
					<p class="text-xs text-red-400">{chatState.channelShowError}</p>
				{/if}
			{/if}
		</div>

		<!-- Footer -->
		<div class="flex shrink-0 justify-end gap-2 border-t border-gray-700/60 px-4 py-3">
			<button
				type="button"
				class="rounded border border-gray-700/60 px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200"
				onclick={() => (chatState.channelShowOpen = false)}
			>
				Cancel
			</button>
			<button
				type="button"
				class="rounded border border-blue-500/40 bg-blue-600/80 px-3 py-1.5 text-xs text-white hover:bg-blue-500 disabled:opacity-50"
				disabled={chatState.channelShowSaving}
				onclick={onConfirm}
			>
				{chatState.channelShowSaving ? 'Saving…' : 'Save'}
			</button>
		</div>
	</div>
</div>
