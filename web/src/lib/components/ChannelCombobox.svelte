<script lang="ts">
	import { KNOWN_GROUPS } from '$lib/api/groups';

	let {
		value = $bindable(''),
		onconfirm,
		onclose,
		onValueChange
	}: {
		value?: string;
		onconfirm: () => void;
		onclose: () => void;
		onValueChange?: () => void;
	} = $props();

	let open = $state(false);
	let activeIndex = $state(-1);

	const MAX_SUGGESTIONS = 20;

	type Item = { value: string; flag: string; label: string; sublabel: string };

	const BROADCAST_ITEM: Item = {
		value: '*',
		flag: '📡',
		label: '*',
		sublabel: 'Broadcast — All'
	};

	const ALL_ITEMS: Item[] = [
		BROADCAST_ITEM,
		...KNOWN_GROUPS.map((g) => ({
			value: g.group,
			flag: g.flag ?? '',
			label: g.group,
			sublabel: `${g.prefix} · ${g.note}`
		}))
	];

	const filtered = $derived((): Item[] => {
		const q = value.trim().toLowerCase();
		if (q === '') return ALL_ITEMS.slice(0, MAX_SUGGESTIONS);
		if (q === '*') return [BROADCAST_ITEM];
		return ALL_ITEMS.filter(
			(item) =>
				item.value.toLowerCase().includes(q) ||
				item.sublabel.toLowerCase().includes(q) ||
				item.flag.includes(q)
		).slice(0, MAX_SUGGESTIONS);
	});

	function select(item: Item) {
		value = item.value;
		open = false;
		activeIndex = -1;
	}

	function handleKeydown(e: KeyboardEvent) {
		const items = filtered();
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			open = true;
			activeIndex = Math.min(activeIndex + 1, items.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			activeIndex = Math.max(activeIndex - 1, -1);
		} else if (e.key === 'Enter') {
			if (activeIndex >= 0 && items[activeIndex]) {
				select(items[activeIndex]);
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
		setTimeout(() => {
			open = false;
			activeIndex = -1;
		}, 150);
	}
</script>

<div class="relative">
	<input
		id="new-channel-value"
		class="w-full rounded-lg border border-ink-dim/30 bg-base px-3 py-2 font-mono text-sm text-ink outline-none placeholder:text-ink-dim focus:border-azure/60"
		placeholder="* or 222"
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

	{#if open && filtered().length > 0}
		<ul
			class="absolute left-0 right-0 top-full z-50 mt-1 max-h-52 overflow-y-auto rounded-lg border border-ink-dim/20 bg-base py-1 shadow-xl"
			role="listbox"
		>
			{#each filtered() as item, i}
				<!-- svelte-ignore a11y_click_events_have_key_events -->
				<li
					role="option"
					aria-selected={i === activeIndex}
					class="flex cursor-pointer items-center gap-2 px-3 py-1.5 {i === activeIndex
						? 'bg-azure/30 text-ink'
						: 'text-ink-muted hover:bg-surface-hi'}"
					onmousedown={() => select(item)}
				>
					<span class="w-5 shrink-0 text-center text-base leading-none">{item.flag}</span>
					<span class="w-14 shrink-0 font-mono text-sm">{item.label}</span>
					<span class="truncate text-xs text-ink-muted">{item.sublabel}</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>
