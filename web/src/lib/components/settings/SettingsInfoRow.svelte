<script lang="ts">
	let {
		label,
		description,
		value,
		mono = false,
		envOverride = false
	}: {
		label: string;
		description?: string;
		value: string;
		mono?: boolean;
		envOverride?: boolean;
	} = $props();
</script>

<div class="flex items-center justify-between gap-4 px-4 py-3">
	<div class="flex min-w-0 flex-1 flex-col gap-0.5">
		<div class="flex items-center gap-1.5">
			<span class="text-xs text-ink-muted">{label}</span>
			{#if envOverride}
				<span class="rounded-full bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-mono font-semibold text-amber-400">env</span>
			{/if}
		</div>
		{#if description}
			<span class="text-[11px] leading-snug text-ink-dim">{description}</span>
		{/if}
	</div>
	<span class={['shrink-0 text-xs text-ink-muted', mono ? 'font-mono' : ''].filter(Boolean).join(' ')}>{value}</span>
</div>
