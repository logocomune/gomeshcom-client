<script lang="ts">
	import { mdiEmailOutline, mdiClose } from '@mdi/js';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { toastStore } from '$lib/stores/toasts.svelte';
</script>

<div class="pointer-events-none fixed bottom-4 right-4 z-[9990] flex flex-col items-end gap-2">
	{#each toastStore.toasts as toast (toast.id)}
		<div
			class="pointer-events-auto flex items-center gap-3 rounded-xl border border-azure/30 bg-surface px-4 py-3 shadow-lg"
			role="status"
			aria-live="polite"
		>
			<span
				class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-azure/40 bg-azure/10 text-azure"
			>
				<MdiIcon path={mdiEmailOutline} size={16} />
			</span>
			<div class="min-w-0">
				<div class="text-xs font-semibold text-ink">Direct Message</div>
				<div class="font-mono text-xs text-azure">{toast.from}</div>
			</div>
			<button
				type="button"
				class="ml-2 flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-ink-dim hover:text-coral"
				aria-label="Dismiss"
				onclick={() => toastStore.dismiss(toast.id)}
			>
				<MdiIcon path={mdiClose} size={14} />
			</button>
		</div>
	{/each}
</div>
