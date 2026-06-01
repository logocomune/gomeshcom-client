<script lang="ts">
	import type { MapPosition } from '$lib/map/types';
	import { formatTime } from '$lib/ui/format';

	let {
		nodes,
		value = $bindable(''),
		placeholder = 'IU5PMP-1',
		onconfirm,
		onclose,
		onValueChange
	}: {
		nodes: MapPosition[];
		value?: string;
		placeholder?: string;
		onconfirm: () => void;
		onclose: () => void;
		onValueChange?: () => void;
	} = $props();

	let open = $state(false);
	let activeIndex = $state(-1);

	const MAX_SUGGESTIONS = 20;

	const filtered = $derived(
		value.trim().length === 0
			? nodes.slice(0, MAX_SUGGESTIONS)
			: nodes
					.filter((n) => n.id.toUpperCase().includes(value.trim().toUpperCase()))
					.slice(0, MAX_SUGGESTIONS)
	);

	function select(node: MapPosition) {
		value = node.id;
		open = false;
		activeIndex = -1;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			open = true;
			activeIndex = Math.min(activeIndex + 1, filtered.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			activeIndex = Math.max(activeIndex - 1, -1);
		} else if (e.key === 'Enter') {
			if (activeIndex >= 0 && filtered[activeIndex]) {
				select(filtered[activeIndex]);
			} else {
				onconfirm();
			}
		} else if (e.key === 'Escape') {
			if (open) {
				open = false;
				activeIndex = -1;
			} else {
				onclose();
			}
		}
	}

	function handleBlur() {
		// Delay so mousedown on item fires before close
		setTimeout(() => {
			open = false;
			activeIndex = -1;
		}, 150);
	}
</script>

<div class="relative">
	<input
		id="new-dm-callsign"
		class="w-full rounded-lg border border-ink-dim/30 bg-base px-3 py-2 font-mono text-sm text-ink outline-none placeholder:text-ink-dim focus:border-azure/60"
		{placeholder}
		autocomplete="off"
		bind:value
		oninput={() => {
			open = true;
			activeIndex = -1;
			onValueChange?.();
		}}
		onfocus={() => {
			open = true;
		}}
		onkeydown={handleKeydown}
		onblur={handleBlur}
	/>

	{#if open && filtered.length > 0}
		<ul
			class="absolute left-0 right-0 top-full z-50 mt-1 max-h-48 overflow-y-auto rounded-lg border border-ink-dim/20 bg-base py-1 shadow-xl"
			role="listbox"
		>
			{#each filtered as node, i}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					role="option"
					aria-selected={i === activeIndex}
					class="flex cursor-pointer items-center justify-between px-3 py-1.5 {i === activeIndex
						? 'bg-azure/30 text-ink'
						: 'text-ink-muted hover:bg-surface-hi'}"
					onmousedown={() => select(node)}
				>
					<span class="font-mono text-sm">{node.id}</span>
					{#if node.lastSeen}
						<span class="text-xs text-ink-muted">{formatTime(node.lastSeen)}</span>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>
