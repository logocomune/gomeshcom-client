<script lang="ts">
	import { onMount } from 'svelte';
	import MdiIcon from '$lib/components/MdiIcon.svelte';
	import SettingsField from '$lib/components/settings/SettingsField.svelte';
	import SettingsInfoRow from '$lib/components/settings/SettingsInfoRow.svelte';
	import { isValidCallsign, normalizeCallsign } from '$lib/ui/callsign';
	import {
		getConfig,
		updateConfig,
		restartApp,
		shutdownApp,
		waitForRestart,
		getStartedAt,
		DemoModeError
	} from '$lib/api/config';
	import type { AppConfig, ConfigPatch } from '$lib/api/config';
	import { connectionState } from '$lib/stores/connection.svelte';
	import {
		mdiTune,
		mdiSend,
		mdiDownload,
		mdiSwapVertical,
		mdiChartBar,
		mdiMessageTextOutline,
		mdiArrowRight,
		mdiInformationOutline
	} from '@mdi/js';

	function formatUptime(seconds: number): string {
		if (seconds <= 0) return '—';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = seconds % 60;
		const parts: string[] = [];
		if (d > 0) parts.push(`${d}d`);
		if (h > 0) parts.push(`${h}h`);
		if (m > 0) parts.push(`${m}m`);
		if (parts.length === 0 || (d === 0 && h === 0)) parts.push(`${s}s`);
		return parts.join(' ');
	}

	function formatStartedAt(iso: string): string {
		if (!iso || iso.startsWith('0001')) return '—';
		try {
			return new Date(iso).toLocaleString();
		} catch {
			return iso;
		}
	}

	let config = $state<AppConfig | null>(null);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let demoMode = $state(false);

	let saveError = $state<string | null>(null);
	let saveSuccess = $state(false);
	let requiresRestart = $state(false);
	let saving = $state(false);
	let restarting = $state(false);
	let restartError = $state<string | null>(null);
	let shuttingDown = $state(false);
	let shutdownError = $state<string | null>(null);

	type Section = 'server' | 'web' | 'storage' | 'system';
	let activeSection = $state<Section>('server');

	// Core
	let myCall = $state('');
	let logLevel = $state('info');
	let httpAddr = $state('');
	let udpListenAddr = $state('');
	let nodeAddr = $state('');
	let maxMsgLen = $state('');
	// Send
	let dedupTtl = $state('');
	// Receive Log
	let receiveLogEnabled = $state(true);
	let receiveLogPath = $state('');
	let receiveLogRetentionDays = $state('');
	let receiveLogReplayWindow = $state('');
	// Request Log
	let requestLogEnabled = $state(false);
	// Stats
	let statsEnabled = $state(true);
	let statsPath = $state('');
	let statsRetentionDays = $state('');
	// Chat Log
	let chatLogPath = $state('');
	let chatLogHistoryWindow = $state('');
	let chatLogMaxHistoryWindow = $state('');

	onMount(async () => {
		await reload();
	});

	async function reload() {
		loading = true;
		loadError = null;
		demoMode = false;
		try {
			config = await getConfig();
			populate(config);
		} catch (err) {
			if (err instanceof DemoModeError) {
				demoMode = true;
			} else {
				loadError = err instanceof Error ? err.message : 'Failed to load config';
			}
		} finally {
			loading = false;
		}
	}

	let isDirty = $derived(
		config !== null &&
			(myCall !== config.my_call.value ||
				logLevel !== config.log_level.value ||
				httpAddr !== config.http_addr.value ||
				udpListenAddr !== config.udp_listen_addr.value ||
				nodeAddr !== config.node_addr.value ||
				maxMsgLen !== String(config.max_message_length.value) ||
				dedupTtl !== config.send.dedup_ttl.value ||
				receiveLogEnabled !== config.receive_log.enabled.value ||
				receiveLogPath !== config.receive_log.path.value ||
				receiveLogRetentionDays !== String(config.receive_log.retention_days.value) ||
				receiveLogReplayWindow !== config.receive_log.replay_window.value ||
				requestLogEnabled !== config.request_log.enabled.value ||
				statsEnabled !== config.stats.enabled.value ||
				statsPath !== config.stats.path.value ||
				statsRetentionDays !== String(config.stats.retention_days.value) ||
				chatLogPath !== config.chat_log.path.value ||
				chatLogHistoryWindow !== config.chat_log.history_window.value ||
				chatLogMaxHistoryWindow !== config.chat_log.max_history_window.value)
	);

	function populate(cfg: AppConfig) {
		myCall = cfg.my_call.value;
		logLevel = cfg.log_level.value;
		httpAddr = cfg.http_addr.value;
		udpListenAddr = cfg.udp_listen_addr.value;
		nodeAddr = cfg.node_addr.value;
		maxMsgLen = String(cfg.max_message_length.value);
		dedupTtl = cfg.send.dedup_ttl.value;
		receiveLogEnabled = cfg.receive_log.enabled.value;
		receiveLogPath = cfg.receive_log.path.value;
		receiveLogRetentionDays = String(cfg.receive_log.retention_days.value);
		receiveLogReplayWindow = cfg.receive_log.replay_window.value;
		requestLogEnabled = cfg.request_log.enabled.value;
		statsEnabled = cfg.stats.enabled.value;
		statsPath = cfg.stats.path.value;
		statsRetentionDays = String(cfg.stats.retention_days.value);
		chatLogPath = cfg.chat_log.path.value;
		chatLogHistoryWindow = cfg.chat_log.history_window.value;
		chatLogMaxHistoryWindow = cfg.chat_log.max_history_window.value;
	}

	async function save() {
		if (!config) return;
		saving = true;
		saveError = null;
		saveSuccess = false;
		requiresRestart = false;

		const normalizedCall = normalizeCallsign(myCall);
		if (!isValidCallsign(normalizedCall)) {
			saveError =
				'Invalid callsign — use 3-10 alphanumeric characters with optional numeric SSID (e.g. IU5PMP-1)';
			saving = false;
			return;
		}

		const maxLen = parseInt(maxMsgLen, 10);
		if (isNaN(maxLen) || maxLen <= 0) {
			saveError = 'Max message length must be a positive integer';
			saving = false;
			return;
		}

		const patch: ConfigPatch = {};

		if (!config.my_call.env_override) patch.my_call = normalizedCall;
		if (!config.log_level.env_override) patch.log_level = logLevel;
		if (!config.http_addr.env_override) patch.http_addr = httpAddr;
		if (!config.udp_listen_addr.env_override) patch.udp_listen_addr = udpListenAddr;
		if (!config.node_addr.env_override) patch.node_addr = nodeAddr;
		if (!config.max_message_length.env_override) patch.max_message_length = maxLen;

		{
			const s = config.send;
			if (!s.dedup_ttl.env_override) {
				patch.send = { dedup_ttl: dedupTtl };
			}
		}

		{
			const rl = config.receive_log;
			if (
				!rl.enabled.env_override ||
				!rl.path.env_override ||
				!rl.retention_days.env_override ||
				!rl.replay_window.env_override
			) {
				patch.receive_log = {};
				if (!rl.enabled.env_override) patch.receive_log.enabled = receiveLogEnabled;
				if (!rl.path.env_override) patch.receive_log.path = receiveLogPath;
				if (!rl.retention_days.env_override) {
					const days = parseInt(receiveLogRetentionDays, 10);
					if (!isNaN(days) && days > 0) patch.receive_log.retention_days = days;
				}
				if (!rl.replay_window.env_override)
					patch.receive_log.replay_window = receiveLogReplayWindow;
			}
		}

		if (!config.request_log.enabled.env_override) {
			patch.request_log = { enabled: requestLogEnabled };
		}

		{
			const s = config.stats;
			if (!s.enabled.env_override || !s.path.env_override || !s.retention_days.env_override) {
				patch.stats = {};
				if (!s.enabled.env_override) patch.stats.enabled = statsEnabled;
				if (!s.path.env_override) patch.stats.path = statsPath;
				if (!s.retention_days.env_override) {
					const days = parseInt(statsRetentionDays, 10);
					if (!isNaN(days) && days > 0) patch.stats.retention_days = days;
				}
			}
		}

		{
			const cl = config.chat_log;
			if (
				!cl.path.env_override ||
				!cl.history_window.env_override ||
				!cl.max_history_window.env_override
			) {
				patch.chat_log = {};
				if (!cl.path.env_override) patch.chat_log.path = chatLogPath;
				if (!cl.history_window.env_override) patch.chat_log.history_window = chatLogHistoryWindow;
				if (!cl.max_history_window.env_override)
					patch.chat_log.max_history_window = chatLogMaxHistoryWindow;
			}
		}

		try {
			const result = await updateConfig(patch);
			config = result.config;
			populate(config);
			requiresRestart = result.requires_restart;
			saveSuccess = true;
			if (patch.my_call && connectionState.stationCallsign !== result.config.my_call.value) {
				connectionState.setStation({ callsign: result.config.my_call.value });
			}
		} catch (err) {
			saveError = err instanceof Error ? err.message : 'Save failed';
		} finally {
			saving = false;
		}
	}

	async function shutdown() {
		shuttingDown = true;
		shutdownError = null;
		try {
			await shutdownApp();
		} catch (err) {
			shutdownError = err instanceof Error ? err.message : 'Shutdown failed';
			shuttingDown = false;
		}
		// On success: process is gone, no further state update needed.
	}

	async function restart() {
		restarting = true;
		restartError = null;
		try {
			const previousStartedAt = await getStartedAt();
			await restartApp();
			await waitForRestart(previousStartedAt);
			window.location.reload();
		} catch (err) {
			restartError = err instanceof Error ? err.message : 'Restart failed';
		} finally {
			restarting = false;
		}
	}
</script>

<div class="min-h-0 flex-1 overflow-y-auto p-4 md:p-6">
	<div class="mx-auto max-w-2xl space-y-5">
		<!-- Header -->
		<div class="rounded-2xl border border-warm/20 bg-surface/70 px-4 py-3.5 backdrop-blur-sm">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2.5">
					<span class="text-warm"><MdiIcon path={mdiTune} size={16} /></span>
					<h1 class="font-mono text-sm font-bold text-azure">Settings goMeshCom</h1>
				</div>
				{#if !loading && !loadError}
					<div class="mt-3 flex gap-1 border-t border-ink-dim/10 pt-3">
						{#each ['server', 'web', 'storage', 'system'] as const as key}
							<button
								onclick={() => (activeSection = key)}
								class="rounded-lg px-3 py-1 text-xs font-medium transition-colors {activeSection ===
								key
									? 'bg-azure/15 text-azure'
									: 'text-ink-muted hover:bg-ink-dim/10 hover:text-ink'}"
							>
								{key === 'server'
									? 'Server'
									: key === 'web'
										? 'Web Interface'
										: key === 'storage'
											? 'Storage'
											: 'System'}
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		{#if loading}
			<p class="px-1 text-sm text-ink-muted">Loading configuration…</p>
		{:else if demoMode}
			<div class="rounded-2xl border border-warm/30 bg-warm/10 p-4 text-sm text-warm">
				<span class="font-semibold">Demo mode active.</span> Configuration is locked. To change
				settings, disable <code class="font-mono">demo_mode</code> in the TOML config file and restart.
			</div>
		{:else if loadError}
			<div class="rounded-2xl border border-coral/30 bg-coral/10 p-4 text-sm text-coral">
				{loadError}
				<button onclick={reload} class="ml-2 underline">Retry</button>
			</div>
		{:else if config}
			{#if restartError}
				<div class="rounded-2xl border border-coral/30 bg-coral/10 p-3 text-sm text-coral">
					{restartError}
				</div>
			{/if}
			{#if shutdownError}
				<div class="rounded-2xl border border-coral/30 bg-coral/10 p-3 text-sm text-coral">
					{shutdownError}
				</div>
			{/if}

			{#if activeSection === 'server'}
				<!-- Core -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiTune} size={14} /></span>
					<span>Core</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="My callsign"
							description="Your station callsign. Changes apply live — no restart needed."
							envOverride={config.my_call.env_override}
							requiresRestart={config.my_call.requires_restart}
						>
							<input
								type="text"
								bind:value={myCall}
								disabled={config.my_call.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="e.g. IU5PMP-1"
							/>
						</SettingsField>
						<SettingsField
							label="Log level"
							description="Verbosity of the application log."
							envOverride={config.log_level.env_override}
							requiresRestart={config.log_level.requires_restart}
						>
							<select
								bind:value={logLevel}
								disabled={config.log_level.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							>
								{#each ['debug', 'info', 'warn', 'error'] as level}
									<option value={level}>{level}</option>
								{/each}
							</select>
						</SettingsField>
						<SettingsField
							label="UDP listen address"
							description="UDP address for incoming MeshCom packets."
							envOverride={config.udp_listen_addr.env_override}
							requiresRestart={config.udp_listen_addr.requires_restart}
						>
							<input
								type="text"
								bind:value={udpListenAddr}
								disabled={config.udp_listen_addr.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="0.0.0.0:1799"
							/>
						</SettingsField>
						<SettingsField
							label="Node address"
							description="Address of the MeshCom node. Leave empty to auto-detect from first incoming packet."
							envOverride={config.node_addr.env_override}
							requiresRestart={config.node_addr.requires_restart}
						>
							<input
								type="text"
								bind:value={nodeAddr}
								disabled={config.node_addr.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="auto-detect"
							/>
						</SettingsField>
						<SettingsField
							label="Max message length"
							description="Maximum outgoing message size in bytes (1–255)."
							envOverride={config.max_message_length.env_override}
							requiresRestart={config.max_message_length.requires_restart}
						>
							<input
								type="number"
								bind:value={maxMsgLen}
								disabled={config.max_message_length.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								min="1"
								max="255"
							/>
						</SettingsField>
					</div>
				</div>

				<!-- Send -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiSend} size={14} /></span>
					<span>Send</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="Dedup TTL"
							description="Suppresses duplicate outgoing messages within this window. Set to 0s to disable."
							envOverride={config.send.dedup_ttl.env_override}
							requiresRestart={config.send.dedup_ttl.requires_restart}
						>
							<input
								type="text"
								bind:value={dedupTtl}
								disabled={config.send.dedup_ttl.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="2s"
							/>
						</SettingsField>
					</div>
				</div>
			{/if}

			{#if activeSection === 'web'}
				<!-- HTTP -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiTune} size={14} /></span>
					<span>HTTP</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="HTTP address"
							description="TCP address the HTTP server binds to."
							envOverride={config.http_addr.env_override}
							requiresRestart={config.http_addr.requires_restart}
						>
							<input
								type="text"
								bind:value={httpAddr}
								disabled={config.http_addr.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="127.0.0.1:8080"
							/>
						</SettingsField>
					</div>
				</div>

				<!-- Request Log -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiSwapVertical} size={14} /></span>
					<span>Request Log</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="Enabled"
							description="Log every HTTP request with method, path, status and duration."
							envOverride={config.request_log.enabled.env_override}
							requiresRestart={config.request_log.enabled.requires_restart}
						>
							<input
								type="checkbox"
								bind:checked={requestLogEnabled}
								disabled={config.request_log.enabled.env_override}
								class="h-4 w-4 cursor-pointer rounded accent-azure disabled:cursor-not-allowed disabled:opacity-50"
							/>
						</SettingsField>
					</div>
				</div>
			{/if}

			{#if activeSection === 'storage'}
				<!-- Receive Log -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiDownload} size={14} /></span>
					<span>Receive Log</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="Enabled"
							description="Write received UDP packets to a daily JSONL log file."
							envOverride={config.receive_log.enabled.env_override}
							requiresRestart={config.receive_log.enabled.requires_restart}
						>
							<input
								type="checkbox"
								bind:checked={receiveLogEnabled}
								disabled={config.receive_log.enabled.env_override}
								class="h-4 w-4 cursor-pointer rounded accent-azure disabled:cursor-not-allowed disabled:opacity-50"
							/>
						</SettingsField>
						<SettingsField
							label="Path"
							description="Directory for daily received UDP JSONL files."
							envOverride={config.receive_log.path.env_override}
							requiresRestart={config.receive_log.path.requires_restart}
						>
							<input
								type="text"
								bind:value={receiveLogPath}
								disabled={config.receive_log.path.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							/>
						</SettingsField>
						<SettingsField
							label="Retention"
							description="How many days of log files to keep."
							envOverride={config.receive_log.retention_days.env_override}
							requiresRestart={config.receive_log.retention_days.requires_restart}
						>
							<input
								type="number"
								bind:value={receiveLogRetentionDays}
								disabled={config.receive_log.retention_days.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								min="1"
							/>
						</SettingsField>
						<SettingsField
							label="Replay window"
							description="Recent packets replayed to new SSE subscribers on connect."
							envOverride={config.receive_log.replay_window.env_override}
							requiresRestart={config.receive_log.replay_window.requires_restart}
						>
							<input
								type="text"
								bind:value={receiveLogReplayWindow}
								disabled={config.receive_log.replay_window.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="5m"
							/>
						</SettingsField>
					</div>
				</div>

				<!-- Stats -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiChartBar} size={14} /></span>
					<span>Stats</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="Enabled"
							description="Collect hourly packet statistics."
							envOverride={config.stats.enabled.env_override}
							requiresRestart={config.stats.enabled.requires_restart}
						>
							<input
								type="checkbox"
								bind:checked={statsEnabled}
								disabled={config.stats.enabled.env_override}
								class="h-4 w-4 cursor-pointer rounded accent-azure disabled:cursor-not-allowed disabled:opacity-50"
							/>
						</SettingsField>
						<SettingsField
							label="Path"
							description="File where statistics are persisted."
							envOverride={config.stats.path.env_override}
							requiresRestart={config.stats.path.requires_restart}
						>
							<input
								type="text"
								bind:value={statsPath}
								disabled={config.stats.path.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							/>
						</SettingsField>
						<SettingsField
							label="Retention"
							description="How many days of hourly buckets to keep."
							envOverride={config.stats.retention_days.env_override}
							requiresRestart={config.stats.retention_days.requires_restart}
						>
							<input
								type="number"
								bind:value={statsRetentionDays}
								disabled={config.stats.retention_days.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								min="1"
							/>
						</SettingsField>
					</div>
				</div>

				<!-- Chat Log -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiMessageTextOutline} size={14} /></span>
					<span>Chat Log</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsField
							label="Path"
							description="Directory for chat JSONL files."
							envOverride={config.chat_log.path.env_override}
							requiresRestart={config.chat_log.path.requires_restart}
						>
							<input
								type="text"
								bind:value={chatLogPath}
								disabled={config.chat_log.path.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							/>
						</SettingsField>
						<SettingsField
							label="History window"
							description="Default time window returned by chat history queries. Changes apply live."
							envOverride={config.chat_log.history_window.env_override}
							requiresRestart={config.chat_log.history_window.requires_restart}
						>
							<input
								type="text"
								bind:value={chatLogHistoryWindow}
								disabled={config.chat_log.history_window.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="24h"
							/>
						</SettingsField>
						<SettingsField
							label="Max history window"
							description="Upper limit for the ?hours= query parameter. Changes apply live."
							envOverride={config.chat_log.max_history_window.env_override}
							requiresRestart={config.chat_log.max_history_window.requires_restart}
						>
							<input
								type="text"
								bind:value={chatLogMaxHistoryWindow}
								disabled={config.chat_log.max_history_window.env_override}
								class="w-full rounded-lg border border-ink-dim/30 bg-base px-2.5 py-1.5 font-mono text-xs text-ink focus:border-azure focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="168h"
							/>
						</SettingsField>
					</div>
				</div>
			{/if}

			{#if activeSection === 'server'}
				<!-- Forward -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiArrowRight} size={14} /></span>
					<span>Forward</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsInfoRow
							label="Targets"
							description="Comma-separated host:port list — incoming UDP packets are mirrored to each target."
							value={config.forward.targets.value || '(none)'}
							envOverride={config.forward.targets.env_override}
						/>
					</div>
				</div>
			{/if}

			{#if activeSection === 'system'}
				<!-- About -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiInformationOutline} size={14} /></span>
					<span>About</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<SettingsInfoRow label="Version" value={config.server.version || '—'} mono />
						<SettingsInfoRow label="Started at" value={formatStartedAt(config.server.started_at)} />
						<SettingsInfoRow
							label="Uptime"
							value={formatUptime(config.server.uptime_seconds)}
							mono
						/>
					</div>
				</div>

				<!-- Restart -->
				<div class="flex items-center gap-2 text-xs uppercase tracking-[0.2em] text-ink-muted">
					<span class="text-ink-muted"><MdiIcon path={mdiTune} size={14} /></span>
					<span>Process</span>
					<span class="h-px flex-1 bg-ink-dim/20"></span>
				</div>
				<div class="-mt-2 rounded-2xl border border-ink-dim/15 bg-surface/50 backdrop-blur-sm">
					<div class="divide-y divide-ink-dim/15">
						<div class="flex items-center justify-between px-4 py-3">
							<div>
								<p class="text-sm font-medium text-ink">Restart</p>
								<p class="mt-0.5 text-xs text-ink-muted">
									Gracefully restart the daemon. Existing connections are closed and the process
									re-executes.
								</p>
							</div>
							<button
								onclick={restart}
								disabled={restarting || shuttingDown}
								class="ml-6 shrink-0 rounded-lg border border-azure/40 px-4 py-1.5 text-sm font-medium text-azure transition-colors hover:bg-azure/10 disabled:cursor-not-allowed disabled:opacity-50"
							>
								{restarting ? 'Restarting…' : 'Restart'}
							</button>
						</div>
						{#if restartError}
							<div class="px-4 py-2 text-xs text-coral">{restartError}</div>
						{/if}
						<div class="flex items-center justify-between px-4 py-3">
							<div>
								<p class="text-sm font-medium text-ink">Shutdown</p>
								<p class="mt-0.5 text-xs text-ink-muted">
									Stop the daemon. The process exits cleanly without restarting.
								</p>
							</div>
							<button
								onclick={shutdown}
								disabled={restarting || shuttingDown}
								class="ml-6 shrink-0 rounded-lg border border-coral/40 px-4 py-1.5 text-sm font-medium text-coral transition-colors hover:bg-coral/10 disabled:cursor-not-allowed disabled:opacity-50"
							>
								{shuttingDown ? 'Shutting down…' : 'Shutdown'}
							</button>
						</div>
						{#if shutdownError}
							<div class="px-4 py-2 text-xs text-coral">{shutdownError}</div>
						{/if}
					</div>
				</div>
			{/if}

			<!-- Bottom Save bar -->
			{#if activeSection !== 'system'}
				<div
					class="flex items-center justify-end gap-3 rounded-2xl border border-ink-dim/15 bg-surface/50 px-4 py-3 backdrop-blur-sm"
				>
					{#if saveSuccess && !isDirty}
						<span class="text-xs text-mint">✓ Saved.</span>
					{/if}
					{#if saveError}
						<span class="text-xs text-coral">{saveError}</span>
					{/if}
					{#if requiresRestart && !isDirty}
						<button
							onclick={restart}
							disabled={restarting || shuttingDown}
							class="rounded-lg border border-amber-500/40 px-4 py-1.5 text-sm font-medium text-amber-400 transition-colors hover:bg-amber-500/10 disabled:cursor-not-allowed disabled:opacity-50"
						>
							{restarting ? 'Restarting…' : 'Restart to apply'}
						</button>
					{/if}
					<button
						onclick={save}
						disabled={!isDirty || saving || restarting || shuttingDown}
						class="rounded-lg bg-azure px-5 py-1.5 text-sm font-medium text-white transition-colors hover:bg-azure/80 disabled:cursor-not-allowed disabled:opacity-40"
					>
						{saving ? 'Saving…' : 'Save'}
					</button>
				</div>
			{/if}
		{/if}
	</div>
</div>
