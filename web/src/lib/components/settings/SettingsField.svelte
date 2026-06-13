<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		label,
		description,
		envOverride,
		requiresRestart,
		children
	}: {
		label: string;
		description?: string;
		envOverride: boolean;
		requiresRestart: boolean;
		children?: Snippet;
	} = $props();
</script>

<div class="flex items-center justify-between gap-4 px-4 py-3">
	<div class="flex min-w-0 flex-1 flex-col gap-0.5">
		<div class="flex items-center gap-1.5">
			<span class="text-xs font-medium text-ink">{label}</span>
			{#if envOverride}
				<span class="group/tooltip relative cursor-help">
					<span class="rounded-full bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-mono font-semibold text-amber-400">
						env
					</span>
					<span class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-64 rounded-xl border border-ink-dim/20 bg-surface px-3 py-2 text-[11px] leading-relaxed text-ink-muted opacity-0 shadow-lg transition-opacity duration-150 group-hover/tooltip:opacity-100">
						Managed by <span class="font-mono text-amber-400">GOMESHCOM_*</span> environment variable.
						Remove it from <span class="font-mono text-ink">docker-compose.yml</span> to allow editing here.
					</span>
				</span>
			{/if}
			{#if requiresRestart}
				<span class="rounded-full bg-ink-dim/20 px-1.5 py-0.5 text-[10px] text-ink-muted">restart</span>
			{/if}
		</div>
		{#if description}
			<span class="text-[11px] leading-snug text-ink-muted">{description}</span>
		{/if}
	</div>
	{#if envOverride}
		<div class="group/input-tooltip relative w-52 shrink-0 cursor-not-allowed">
			{@render children?.()}
			<span class="pointer-events-none absolute bottom-full right-0 z-50 mb-2 w-64 rounded-xl border border-ink-dim/20 bg-surface px-3 py-2 text-[11px] leading-relaxed text-ink-muted opacity-0 shadow-lg transition-opacity duration-150 group-hover/input-tooltip:opacity-100">
				Managed by <span class="font-mono text-amber-400">GOMESHCOM_*</span> environment variable.
				Remove it from <span class="font-mono text-ink">docker-compose.yml</span> to allow editing here.
			</span>
		</div>
	{:else}
		<div class="w-52 shrink-0">
			{@render children?.()}
		</div>
	{/if}
</div>
