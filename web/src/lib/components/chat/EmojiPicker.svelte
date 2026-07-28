<script lang="ts">
	import { tick } from 'svelte';
	import { mdiClose } from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { emojiTabs } from '$lib/ui/emoji-catalog';

	interface Props {
		disabled?: boolean;
		onSelect: (emoji: string) => void;
		onClose: () => void;
	}

	let { disabled = false, onSelect, onClose }: Props = $props();
	let closeButton = $state<HTMLButtonElement | null>(null);
	let selectedCategoryId = $state(emojiTabs[0].id);
	let selectedCategory = $derived(
		emojiTabs.find((category) => category.id === selectedCategoryId) ?? emojiTabs[0]
	);

	$effect(() => {
		void tick().then(() => closeButton?.focus());
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') onClose();
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-end justify-center bg-black/50 p-3 sm:items-center"
	role="presentation"
	onclick={onClose}
	onkeydown={handleKeydown}
>
	<div
		class="emoji-picker-panel max-h-[80vh] w-full max-w-xl overflow-y-auto rounded-xl border border-ink-dim/30 bg-surface p-3 shadow-xl"
		role="dialog"
		tabindex="-1"
		aria-modal="true"
		aria-label="Choose an emoji"
		onclick={(event) => event.stopPropagation()}
		onkeydown={handleKeydown}
	>
		<div class="mb-2 flex items-center justify-between">
			<h2 class="text-sm font-semibold text-ink">Emoji</h2>
			<button
				bind:this={closeButton}
				class="rounded-md p-1 text-ink-muted hover:bg-ink-dim/10 hover:text-ink"
				type="button"
				aria-label="Close emoji picker"
				onclick={onClose}
			>
				<MdiIcon path={mdiClose} size={18} />
			</button>
		</div>
		<div class="mb-3 flex gap-1 overflow-x-auto pb-1" role="tablist" aria-label="Emoji categories">
			{#each emojiTabs as category}
				<button
					class="shrink-0 rounded-md px-2 py-1 text-xs {selectedCategory.id === category.id
						? 'bg-azure/20 text-ink'
						: 'text-ink-muted hover:bg-ink-dim/10 hover:text-ink'}"
					type="button"
					role="tab"
					aria-selected={selectedCategory.id === category.id}
					onclick={() => (selectedCategoryId = category.id)}
				>
					{category.label}
				</button>
			{/each}
		</div>
		<div class="grid grid-cols-8 gap-1" role="tabpanel" aria-label={selectedCategory.label}>
			{#each selectedCategory.emojis as emoji}
				<button
					class="rounded-md p-1.5 text-xl hover:bg-azure/15 disabled:cursor-not-allowed disabled:opacity-50"
					type="button"
					{disabled}
					aria-label={emoji}
					onclick={() => onSelect(emoji)}
				>
					{emoji}
				</button>
			{/each}
		</div>
	</div>
</div>

<style>
	.emoji-picker-panel {
		scrollbar-color: rgb(82 174 255 / 0.55) transparent;
		scrollbar-width: thin;
	}

	.emoji-picker-panel::-webkit-scrollbar {
		width: 8px;
	}

	.emoji-picker-panel::-webkit-scrollbar-track {
		margin: 10px 0;
		background: transparent;
	}

	.emoji-picker-panel::-webkit-scrollbar-thumb {
		border: 2px solid transparent;
		border-radius: 999px;
		background: rgb(82 174 255 / 0.5);
		background-clip: padding-box;
	}

	.emoji-picker-panel::-webkit-scrollbar-thumb:hover {
		background: rgb(82 174 255 / 0.8);
		background-clip: padding-box;
	}
</style>
