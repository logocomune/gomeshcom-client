<script lang="ts">
	import type { AckIndex } from '$lib/api/acks';
	import ChatList from '$lib/components/chat/ChatList.svelte';
	import ChatThread from '$lib/components/chat/ChatThread.svelte';
	import { chatState } from '$lib/stores/chat.svelte';

	interface Props {
		ackIndex: AckIndex;
		handleSend: () => void | Promise<void>;
		isDesktop: boolean;
	}

	let { ackIndex, handleSend, isDesktop }: Props = $props();

	// Mobile: toggle between list and thread views
	let showThread = $state(false);

	function openThread() {
		showThread = true;
	}

	function closeThread() {
		showThread = false;
	}

	function startSidebarDrag(e: PointerEvent) {
		e.preventDefault();
		const startX = e.clientX;
		const startW = chatState.chatListWidthPx;

		function onMove(ev: PointerEvent) {
			chatState.setChatListWidth(startW + (ev.clientX - startX));
		}
		function onUp() {
			chatState.saveChatListWidth();
			window.removeEventListener('pointermove', onMove);
			window.removeEventListener('pointerup', onUp);
		}
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp);
	}
</script>

{#if isDesktop}
	<!-- Desktop: resizable sidebar list + thread side by side -->
	<div class="flex min-h-0 flex-1 overflow-hidden">
		<aside
			class="shrink-0 border-r border-gray-700/60 overflow-hidden"
			style="width: {chatState.chatListWidthPx}px"
		>
			<ChatList />
		</aside>
		<!-- Drag handle -->
		<div
			role="separator"
			aria-orientation="vertical"
			aria-label="Resize chat list"
			class="group relative z-10 mx-0.5 flex w-2 shrink-0 cursor-col-resize items-center justify-center"
			onpointerdown={startSidebarDrag}
		>
			<div
				class="h-12 w-0.5 rounded-full bg-gray-700/60 transition-colors group-hover:bg-blue-500/60 group-active:bg-blue-500"
			></div>
		</div>
		<div class="flex min-h-0 flex-1 flex-col bg-[#212735]">
			<ChatThread {ackIndex} {handleSend} />
		</div>
	</div>
{:else}
	<!-- Mobile: show list OR thread, full screen -->
	<div class="flex min-h-0 flex-1 flex-col overflow-hidden bg-[#1c2230]">
		{#if showThread}
			<ChatThread {ackIndex} {handleSend} showBack={true} onBack={closeThread} />
		{:else}
			<ChatList onSelectConversation={openThread} />
		{/if}
	</div>
{/if}
