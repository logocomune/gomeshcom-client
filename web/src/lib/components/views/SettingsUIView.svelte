<script lang="ts">
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import { uiPrefs } from '$lib/stores/ui-prefs.svelte';
	import {
		mdiBellOutline,
		mdiCheckCircleOutline,
		mdiClockOutline,
		mdiFormatListBulletedSquare,
		mdiMonitor,
		mdiPalette,
		mdiRestoreAlert
	} from '@mdi/js';

	let saveSuccess = $state(false);

	function save() {
		uiPrefs.save();
		saveSuccess = true;
		setTimeout(() => (saveSuccess = false), 2000);
	}

	function reset() {
		uiPrefs.reset();
		saveSuccess = true;
		setTimeout(() => (saveSuccess = false), 2000);
	}
</script>

<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
	<div class="flex h-11 shrink-0 items-center gap-2 border-b border-ink-dim/20 px-4">
		<span
			class="flex h-7 w-7 items-center justify-center rounded-lg border border-azure/40 bg-azure/10 text-azure"
		>
			<MdiIcon path={mdiPalette} size={16} />
		</span>
		<h1 class="text-sm font-semibold text-ink">Interface Settings</h1>
	</div>

	<div class="min-h-0 flex-1 overflow-y-auto p-4">
		<div class="mx-auto max-w-2xl space-y-4">
			<!-- Notifications -->
			<section class="rounded-xl border border-ink-dim/20 bg-surface-soft">
				<div class="flex items-center gap-2 border-b border-ink-dim/20 px-4 py-2.5">
					<span class="text-ink-muted"><MdiIcon path={mdiBellOutline} size={15} /></span>
					<span class="text-xs font-semibold uppercase tracking-wider text-ink-muted"
						>Notifications</span
					>
				</div>
				<div class="divide-y divide-ink-dim/10">
					<label class="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
						<div>
							<div class="text-sm text-ink">Sound alerts</div>
							<div class="text-xs text-ink-muted">Play audio tone on incoming packets</div>
						</div>
						<button
							type="button"
							role="switch"
							aria-label="Toggle sound alerts"
							aria-checked={uiPrefs.soundEnabled}
							onclick={() => (uiPrefs.soundEnabled = !uiPrefs.soundEnabled)}
							class="inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors
								{uiPrefs.soundEnabled ? 'bg-azure' : 'bg-ink-dim/40'}"
						>
							<span
								class="ml-0.5 h-4 w-4 shrink-0 rounded-full bg-white shadow transition-transform
									{uiPrefs.soundEnabled ? 'translate-x-4' : 'translate-x-0'}"
							></span>
						</button>
					</label>
					<label class="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
						<div>
							<div class="text-sm text-ink">DM sound alert</div>
							<div class="text-xs text-ink-muted">
								Play distinct ping on incoming direct messages
							</div>
						</div>
						<button
							type="button"
							role="switch"
							aria-label="Toggle DM sound alert"
							aria-checked={uiPrefs.dmSoundEnabled}
							onclick={() => (uiPrefs.dmSoundEnabled = !uiPrefs.dmSoundEnabled)}
							class="inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors
								{uiPrefs.dmSoundEnabled ? 'bg-azure' : 'bg-ink-dim/40'}"
						>
							<span
								class="ml-0.5 h-4 w-4 shrink-0 rounded-full bg-white shadow transition-transform
									{uiPrefs.dmSoundEnabled ? 'translate-x-4' : 'translate-x-0'}"
							></span>
						</button>
					</label>
					<label class="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
						<div>
							<div class="text-sm text-ink">DM toast notification</div>
							<div class="text-xs text-ink-muted">
								Show banner with sender callsign on incoming direct messages
							</div>
						</div>
						<button
							type="button"
							role="switch"
							aria-label="Toggle DM toast notification"
							aria-checked={uiPrefs.dmToastEnabled}
							onclick={() => (uiPrefs.dmToastEnabled = !uiPrefs.dmToastEnabled)}
							class="inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors
								{uiPrefs.dmToastEnabled ? 'bg-azure' : 'bg-ink-dim/40'}"
						>
							<span
								class="ml-0.5 h-4 w-4 shrink-0 rounded-full bg-white shadow transition-transform
									{uiPrefs.dmToastEnabled ? 'translate-x-4' : 'translate-x-0'}"
							></span>
						</button>
					</label>
					<label class="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
						<div>
							<div class="text-sm text-ink">Mention toast notification</div>
							<div class="text-xs text-ink-muted">
								Show banner when your callsign is @mentioned in a channel
							</div>
						</div>
						<button
							type="button"
							role="switch"
							aria-label="Toggle mention toast notification"
							aria-checked={uiPrefs.mentionToastEnabled}
							onclick={() => (uiPrefs.mentionToastEnabled = !uiPrefs.mentionToastEnabled)}
							class="inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors
								{uiPrefs.mentionToastEnabled ? 'bg-azure' : 'bg-ink-dim/40'}"
						>
							<span
								class="ml-0.5 h-4 w-4 shrink-0 rounded-full bg-white shadow transition-transform
									{uiPrefs.mentionToastEnabled ? 'translate-x-4' : 'translate-x-0'}"
							></span>
						</button>
					</label>
				</div>
			</section>

			<!-- Display -->
			<section class="rounded-xl border border-ink-dim/20 bg-surface-soft">
				<div class="flex items-center gap-2 border-b border-ink-dim/20 px-4 py-2.5">
					<span class="text-ink-muted"><MdiIcon path={mdiMonitor} size={15} /></span>
					<span class="text-xs font-semibold uppercase tracking-wider text-ink-muted">Display</span>
				</div>
				<div class="divide-y divide-ink-dim/10">
					<label class="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
						<div>
							<div class="text-sm text-ink">Compact mode</div>
							<div class="text-xs text-ink-muted">Reduce padding and spacing in lists</div>
						</div>
						<button
							type="button"
							role="switch"
							aria-label="Toggle compact mode"
							aria-checked={uiPrefs.compactMode}
							onclick={() => (uiPrefs.compactMode = !uiPrefs.compactMode)}
							class="inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors
								{uiPrefs.compactMode ? 'bg-azure' : 'bg-ink-dim/40'}"
						>
							<span
								class="ml-0.5 h-4 w-4 shrink-0 rounded-full bg-white shadow transition-transform
									{uiPrefs.compactMode ? 'translate-x-4' : 'translate-x-0'}"
							></span>
						</button>
					</label>

					<label class="flex cursor-pointer items-center justify-between gap-4 px-4 py-3">
						<div>
							<div class="text-sm text-ink">Packet counter</div>
							<div class="text-xs text-ink-muted">Show received packet count in header</div>
						</div>
						<button
							type="button"
							role="switch"
							aria-label="Toggle packet counter"
							aria-checked={uiPrefs.showPacketCounter}
							onclick={() => (uiPrefs.showPacketCounter = !uiPrefs.showPacketCounter)}
							class="inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors
								{uiPrefs.showPacketCounter ? 'bg-azure' : 'bg-ink-dim/40'}"
						>
							<span
								class="ml-0.5 h-4 w-4 shrink-0 rounded-full bg-white shadow transition-transform
									{uiPrefs.showPacketCounter ? 'translate-x-4' : 'translate-x-0'}"
							></span>
						</button>
					</label>
				</div>
			</section>

			<!-- Timestamps -->
			<section class="rounded-xl border border-ink-dim/20 bg-surface-soft">
				<div class="flex items-center gap-2 border-b border-ink-dim/20 px-4 py-2.5">
					<span class="text-ink-muted"><MdiIcon path={mdiClockOutline} size={15} /></span>
					<span class="text-xs font-semibold uppercase tracking-wider text-ink-muted"
						>Timestamps</span
					>
				</div>
				<div class="divide-y divide-ink-dim/10">
					<div class="px-4 py-3">
						<div class="mb-2 text-sm text-ink">Format</div>
						<div class="flex gap-2">
							<button
								type="button"
								onclick={() => (uiPrefs.timestampFormat = 'absolute')}
								class="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs transition-colors
									{uiPrefs.timestampFormat === 'absolute'
									? 'border-azure/60 bg-azure/10 text-azure'
									: 'border-ink-dim/30 text-ink-muted hover:text-ink'}"
							>
								<MdiIcon path={mdiFormatListBulletedSquare} size={14} />
								Absolute
							</button>
							<button
								type="button"
								onclick={() => (uiPrefs.timestampFormat = 'relative')}
								class="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs transition-colors
									{uiPrefs.timestampFormat === 'relative'
									? 'border-azure/60 bg-azure/10 text-azure'
									: 'border-ink-dim/30 text-ink-muted hover:text-ink'}"
							>
								<MdiIcon path={mdiClockOutline} size={14} />
								Relative
							</button>
						</div>
						<p class="mt-1.5 text-xs text-ink-muted">
							{uiPrefs.timestampFormat === 'absolute'
								? 'Shows exact time — e.g. 14:32:05'
								: 'Shows relative time — e.g. 2 minutes ago'}
						</p>
					</div>
				</div>
			</section>

			<!-- Actions -->
			<div class="flex items-center justify-between">
				<button
					type="button"
					onclick={reset}
					class="flex items-center gap-1.5 rounded-lg border border-ink-dim/30 px-3 py-1.5 text-xs text-ink-muted transition-colors hover:border-coral/40 hover:text-coral"
				>
					<MdiIcon path={mdiRestoreAlert} size={14} />
					Reset to defaults
				</button>

				<div class="flex items-center gap-2">
					{#if saveSuccess}
						<span class="flex items-center gap-1 text-xs text-mint">
							<MdiIcon path={mdiCheckCircleOutline} size={14} />
							Saved
						</span>
					{/if}
					<button
						type="button"
						onclick={save}
						class="rounded-lg bg-azure/80 px-4 py-1.5 text-xs font-medium text-base transition-colors hover:bg-azure"
					>
						Save
					</button>
				</div>
			</div>
		</div>
	</div>
</div>
