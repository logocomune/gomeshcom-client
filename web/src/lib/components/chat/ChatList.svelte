<script lang="ts">
	import { mdiBroadcast, mdiCog, mdiPlus, mdiPound } from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import PixelAvatar from '$lib/components/PixelAvatar.svelte';
	import { chatState } from '$lib/stores/chat.svelte';
	import { conversationIdFor } from '$lib/api/chat';
	import { resolveGroup, type MeshcomGroup } from '$lib/api/groups';
	import { sortByRecency, previewText, conversationPreview, formatRelativeTime } from '$lib/ui/chat-list';
	import type { Conversation } from '$lib/api/types';

	interface Props {
		onSelectConversation?: () => void;
	}

	let { onSelectConversation }: Props = $props();

	let channels = $derived(
		sortByRecency(chatState.visibleConversations.filter((c) => c.kind !== 'dm'))
	);
	let dms = $derived(
		sortByRecency(chatState.conversations.filter((c) => c.kind === 'dm'))
	);

	function groupFor(conv: Conversation): MeshcomGroup | null {
		if (conv.kind !== 'channel' || conv.id === 'P_broadcast') return null;
		return resolveGroup(conv.label);
	}

	function iconFor(conv: Conversation) {
		if (conv.id === 'P_broadcast') return mdiBroadcast;
		return mdiPound;
	}

	function displayName(conv: Conversation): string {
		if (conv.id === 'P_broadcast') return 'Broadcast';
		const group = groupFor(conv);
		if (group) return `${group.note} · ${conv.label}`;
		return conv.label;
	}

	function handleSelect(conv: Conversation) {
		if (conv.kind === 'dm') {
			chatState.selectContact(conv.label);
		} else {
			chatState.selectChannel(conv.label === 'Broadcast' ? 'Broadcast' : conv.label);
		}
		onSelectConversation?.();
	}

	function isActiveConv(conv: Conversation): boolean {
		if (conv.kind === 'dm') {
			return conv.id === conversationIdFor({ kind: 'contact', value: chatState.chatTarget.value });
		}
		return conv.id === conversationIdFor({ kind: 'channel', value: chatState.chatTarget.value });
	}
</script>

<div class="flex h-full flex-col bg-[#1c2230]">
	<div class="flex h-10 shrink-0 items-center border-b border-gray-700/60 px-3">
		<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-400">Chats</span>
	</div>

	<div class="min-h-0 flex-1 overflow-y-auto">
		<!-- Channels section -->
		<div class="flex items-center justify-between px-3 py-1.5">
			<span class="text-[10px] font-semibold uppercase tracking-wider text-gray-500">Channels</span>
			<div class="flex items-center gap-1">
				<button
					type="button"
					class="flex h-5 w-5 items-center justify-center rounded text-gray-500 hover:bg-gray-700/40 hover:text-gray-300"
					title="Channel visibility settings"
					onclick={() => chatState.openChannelShowModal()}
				>
					<MdiIcon path={mdiCog} size={13} />
				</button>
				<button
					type="button"
					class="flex items-center gap-1 rounded bg-blue-600 px-2 py-0.5 text-[10px] font-semibold text-white hover:bg-blue-500 active:bg-blue-700"
					title="Join Channel"
					onclick={() => {
						chatState.newChannelValue = '';
						chatState.newChannelError = '';
						chatState.newChannelOpen = true;
					}}
				>
					<MdiIcon path={mdiPlus} size={12} />
					Add Channel
				</button>
			</div>
		</div>
		{#if channels.length > 0}
			{#each channels as conv (conv.id)}
				{@const statusMsg = chatState.chatStatus[conv.id]?.lastMsg}
				{@const preview = statusMsg != null ? previewText(statusMsg) : conversationPreview(chatState.chatHistory[conv.id] ?? [])}
				{@const count = chatState.chatStatus[conv.id]?.unreadCount ?? 0}
				{@const relTime = formatRelativeTime(conv.last_seen)}
				{@const active = isActiveConv(conv)}
				{@const group = groupFor(conv)}
				<button
					type="button"
					class="flex w-full items-center gap-3 border-b border-gray-700/30 px-3 py-3 text-left transition-colors
						{active ? 'bg-blue-500/10' : 'hover:bg-gray-700/25'}"
					onclick={() => handleSelect(conv)}
				>
					<span class="relative shrink-0">
						<span
							class="flex h-9 w-9 items-center justify-center rounded-full border
								{active ? 'border-blue-500/50 bg-blue-500/20 text-blue-300' : 'border-gray-700/60 bg-[#212735] text-gray-400'}"
						>
							{#if group}
								<span class="text-lg leading-none">{group.flag}</span>
							{:else}
								<MdiIcon path={iconFor(conv)} size={18} />
							{/if}
						</span>
						{#if count > 0}
							<span class="absolute -right-1 -top-1 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-blue-500 px-1 text-[10px] font-bold text-white">
								{count > 99 ? '99+' : count}
							</span>
						{/if}
					</span>
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between gap-2">
							<span class="truncate text-sm {count > 0 ? 'font-semibold text-white' : 'font-medium text-gray-200'}">
								{displayName(conv)}
							</span>
							{#if relTime}
								<span class="shrink-0 text-[11px] text-gray-500">{relTime}</span>
							{/if}
						</div>
						<div class="flex items-center justify-between gap-2">
							<span class="truncate text-xs text-gray-500">{preview || ' '}</span>
						</div>
					</div>
				</button>
			{/each}
		{/if}

		<!-- Direct Messages section -->
		<div class="flex items-center justify-between border-t border-gray-700/60 px-3 py-1.5">
			<span class="text-[10px] font-semibold uppercase tracking-wider text-gray-500">Direct Messages</span>
			<button
				type="button"
				class="flex items-center gap-1 rounded bg-blue-600 px-2 py-0.5 text-[10px] font-semibold text-white hover:bg-blue-500 active:bg-blue-700"
				title="New Direct Message"
				onclick={() => {
					chatState.newDmCallsign = '';
					chatState.newDmError = '';
					chatState.newDmOpen = true;
				}}
			>
				<MdiIcon path={mdiPlus} size={12} />
				Add DM
			</button>
		</div>
		{#if dms.length > 0}
				{#each dms as conv (conv.id)}
					{@const statusMsg = chatState.chatStatus[conv.id]?.lastMsg}
					{@const preview = statusMsg != null ? previewText(statusMsg) : conversationPreview(chatState.chatHistory[conv.id] ?? [])}
					{@const count = chatState.chatStatus[conv.id]?.unreadCount ?? 0}
					{@const relTime = formatRelativeTime(conv.last_seen)}
					{@const active = isActiveConv(conv)}
					<button
						type="button"
						class="flex w-full items-center gap-3 border-b border-gray-700/30 px-3 py-3 text-left transition-colors
							{active ? 'bg-blue-500/10' : 'hover:bg-gray-700/25'}"
						onclick={() => handleSelect(conv)}
					>
						<span class="relative shrink-0">
							<span
								class="block overflow-hidden rounded-full border
									{active ? 'border-blue-500/50' : 'border-gray-700/60'}"
							>
								<PixelAvatar seed={conv.label} size={36} />
							</span>
							{#if count > 0}
								<span class="absolute -right-1 -top-1 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-blue-500 px-1 text-[10px] font-bold text-white">
									{count > 99 ? '99+' : count}
								</span>
							{/if}
						</span>
						<div class="min-w-0 flex-1">
							<div class="flex items-center justify-between gap-2">
								<span class="truncate text-sm {count > 0 ? 'font-semibold text-white' : 'font-medium text-gray-200'}">
									{conv.label}
								</span>
								{#if relTime}
									<span class="shrink-0 text-[11px] text-gray-500">{relTime}</span>
								{/if}
							</div>
							<div class="flex items-center justify-between gap-2">
								<span class="truncate text-xs text-gray-500">{preview || ' '}</span>
							</div>
						</div>
					</button>
				{/each}
		{/if}
	</div>

</div>
