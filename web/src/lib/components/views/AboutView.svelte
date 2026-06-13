<script lang="ts">
	import { onMount } from 'svelte';
	import { connectionState } from '$lib/stores/connection.svelte';
	import { base } from '$app/paths';

	let version = $state('…');
	let healthCallsign = $state('');

	onMount(async () => {
		try {
			const res = await fetch('/api/health');
			if (res.ok) {
				const data = await res.json();
				if (data.version) version = data.version;
				if (data.callsign) healthCallsign = data.callsign;
			}
		} catch {
			// server unreachable
		}
	});

	let callsign = $derived(connectionState.stationCallsign || healthCallsign);
</script>

<div class="min-h-0 flex-1 overflow-y-auto bg-base px-4 py-8">
	<div class="mx-auto max-w-xl space-y-6">
		<div class="rounded-2xl border border-azure/30 bg-azure/5 p-6">
			<h1 class="font-mono text-2xl font-bold text-azure">goMeshCom</h1>
			<p class="mt-1 text-sm text-ink-muted">Web client for the MeshCom mesh radio network</p>
			{#if version !== '…'}
				<dl class="mt-4 grid gap-3 text-xs text-azure sm:grid-cols-2">
					<div>
						<dt class="text-azure/70">Version</dt>
						<dd class="font-mono">{version}</dd>
					</div>
					<div>
						<dt class="text-azure/70">My call</dt>
						<dd class="flex items-center gap-2">
							<span class="font-mono">{callsign || '(unset)'}</span>
							<a
								href="{base}/settings"
								class="rounded border border-ink-dim/30 px-1.5 py-0.5 text-xs text-ink-muted hover:border-azure/50 hover:text-azure"
							>
								Edit in Settings
							</a>
						</dd>
					</div>
				</dl>
			{/if}
		</div>

		<div class="rounded-2xl border border-ink-dim/20 bg-surface p-5 space-y-3">
			<h2 class="text-sm font-semibold text-ink">Bug Reports &amp; Improvements</h2>
			<p class="text-xs text-ink-muted">
				Open an issue on GitHub to report a bug, request a feature, or suggest an improvement.
			</p>
			<a
				href="https://github.com/logocomune/gomeshcom-client/issues"
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1.5 rounded-lg border border-ink-dim/30 bg-surface-hi px-3 py-1.5 text-xs text-ink hover:border-azure/50 hover:text-azure"
			>
				<svg class="h-3.5 w-3.5" viewBox="0 0 16 16" fill="currentColor">
					<path
						d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z"
					/>
				</svg>
				GitHub Issues
			</a>
		</div>

		<div class="rounded-2xl border border-ink-dim/20 bg-surface p-5 space-y-3">
			<h2 class="text-sm font-semibold text-ink">Reference Repository</h2>
			<p class="text-xs text-ink-muted">Upstream codebase used as implementation reference.</p>
			<a
				href="https://github.com/logocomune/gomeshcom-client"
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1.5 rounded-lg border border-ink-dim/30 bg-surface-hi px-3 py-1.5 text-xs text-ink hover:border-azure/50 hover:text-azure"
			>
				github.com/logocomune/gomeshcom-client
			</a>
		</div>

		<div class="rounded-2xl border border-ink-dim/20 bg-surface p-5 space-y-3">
			<h2 class="text-sm font-semibold text-ink">Author &amp; Contact</h2>
			<p class="text-xs text-ink-muted">Developed by Alessandro (IU5PMP). For direct inquiries:</p>
			<a
				href="mailto:alessandro.lucaferro@gmail.com"
				class="font-mono text-xs text-azure hover:text-azure/80"
			>
				alessandro.lucaferro@gmail.com
			</a>
		</div>

		<div class="rounded-2xl border border-ink-dim/20 bg-surface p-5">
			<h2 class="text-sm font-semibold text-ink">Disclaimer</h2>
			<p class="mt-2 text-xs leading-relaxed text-ink-muted">
				This software is provided "as is", without warranty of any kind. The author is not liable
				for any damage arising from its use. Use of this software for radio communications is
				subject to the regulations of your national telecommunications authority.
			</p>
		</div>
	</div>
</div>
