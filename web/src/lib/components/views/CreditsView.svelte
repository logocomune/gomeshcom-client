<script lang="ts">
	const goLibs = [
		{ name: 'ardanlabs/conf', version: 'v3.11.0', url: 'https://github.com/ardanlabs/conf', license: 'MIT', desc: 'Environment and CLI configuration' },
		{ name: 'hashicorp/golang-lru', version: 'v2.0.7', url: 'https://github.com/hashicorp/golang-lru', license: 'MPL-2.0', desc: 'LRU cache with TTL expiry' },
		{ name: 'modernc.org/sqlite', version: 'v1.53.0', url: 'https://pkg.go.dev/modernc.org/sqlite', license: 'BSD-3-Clause', desc: 'Pure Go SQLite driver for persistent storage' }
	];

	const jsLibs = [
		{ name: 'Svelte / SvelteKit', version: '5.x / 2.x', url: 'https://svelte.dev', license: 'MIT', desc: 'UI framework and application shell' },
		{ name: 'OpenLayers', version: '10.x', url: 'https://openlayers.org', license: 'BSD-2-Clause', desc: 'Interactive map engine' },
		{ name: 'ol-ext', version: '4.x', url: 'https://viglino.github.io/ol-ext', license: 'BSD-3-Clause', desc: 'OpenLayers extensions (Maidenhead grid, night overlay)' },
		{ name: 'Material Design Icons (@mdi/js)', version: '7.x', url: 'https://pictogrammers.com/library/mdi', license: 'Apache-2.0', desc: 'Icon set' },
		{ name: 'Tailwind CSS', version: '4.x', url: 'https://tailwindcss.com', license: 'MIT', desc: 'Utility-first CSS framework' },
		{ name: 'Vite', version: '8.x', url: 'https://vite.dev', license: 'MIT', desc: 'Build tool and dev server' },
		{ name: 'TypeScript', version: '6.x', url: 'https://www.typescriptlang.org', license: 'Apache-2.0', desc: 'Typed JavaScript' },
		{ name: 'fast-check', version: '4.x', url: 'https://fast-check.dev', license: 'MIT', desc: 'Property-based testing' },
		{ name: 'Vitest', version: '4.x', url: 'https://vitest.dev', license: 'MIT', desc: 'Unit test framework' },
		{ name: 'Playwright', version: '1.x', url: 'https://playwright.dev', license: 'Apache-2.0', desc: 'End-to-end test framework' },
		{ name: 'Prettier', version: '3.x', url: 'https://prettier.io', license: 'MIT', desc: 'Code formatter' },
		{ name: 'ESLint', version: '10.x', url: 'https://eslint.org', license: 'MIT', desc: 'JavaScript / TypeScript linter' }
	];
</script>

<div class="min-h-0 flex-1 overflow-y-auto bg-base px-4 py-8">
	<div class="mx-auto max-w-2xl">

		<h1 class="mb-1 font-mono text-2xl font-bold text-azure">gomeshcom</h1>
		<p class="mb-8 text-sm text-ink-muted">
			A MeshCom UDP gateway client — open-source software built on open-source shoulders.
		</p>

		<section class="mb-10 rounded-2xl border border-azure/30 bg-azure/5 p-5">
			<h2 class="mb-2 text-xs font-semibold uppercase tracking-wider text-azure">MeshCom Project</h2>
			<p class="mb-3 text-sm text-ink-muted">
				This software interfaces with the <strong class="text-ink">MeshCom</strong> LoRa mesh network
				developed by OE1KBC and the Austrian amateur radio community (ÖVSV / ICSSW).
			</p>
			<a
				href="https://icssw.org/en/meshcom/"
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1.5 rounded-lg border border-azure/40 bg-azure/10 px-3 py-1.5 text-xs font-medium text-azure hover:bg-azure/20"
			>
				icssw.org/en/meshcom ↗
			</a>
		</section>

		<section class="mb-8">
			<h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-ink-muted">Go backend libraries</h2>
			<div class="flex flex-col gap-2">
				<div class="rounded-2xl border border-ink-dim/20 bg-surface px-4 py-3">
					<div class="flex items-baseline justify-between gap-2">
						<a href="https://go.dev" target="_blank" rel="noopener noreferrer"
							class="font-mono text-sm font-semibold text-ink hover:text-azure">Go</a>
						<span class="shrink-0 text-xs text-ink-muted">1.26 · BSD-3-Clause</span>
					</div>
					<p class="mt-0.5 text-xs text-ink-muted">Language runtime, standard library, toolchain</p>
				</div>
				{#each goLibs as lib (lib.name)}
					<div class="rounded-2xl border border-ink-dim/20 bg-surface px-4 py-3">
						<div class="flex items-baseline justify-between gap-2">
							<a href={lib.url} target="_blank" rel="noopener noreferrer"
								class="font-mono text-sm font-semibold text-ink hover:text-azure">{lib.name}</a>
							<span class="shrink-0 text-xs text-ink-muted">{lib.version} · {lib.license}</span>
						</div>
						<p class="mt-0.5 text-xs text-ink-muted">{lib.desc}</p>
					</div>
				{/each}
			</div>
		</section>

		<section class="mb-8">
			<h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-ink-muted">Frontend libraries</h2>
			<div class="flex flex-col gap-2">
				{#each jsLibs as lib (lib.name)}
					<div class="rounded-2xl border border-ink-dim/20 bg-surface px-4 py-3">
						<div class="flex items-baseline justify-between gap-2">
							<a href={lib.url} target="_blank" rel="noopener noreferrer"
								class="font-mono text-sm font-semibold text-ink hover:text-azure">{lib.name}</a>
							<span class="shrink-0 text-xs text-ink-muted">{lib.version} · {lib.license}</span>
						</div>
						<p class="mt-0.5 text-xs text-ink-muted">{lib.desc}</p>
					</div>
				{/each}
			</div>
		</section>

		<section class="mb-8">
			<h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-ink-muted">Map data</h2>
			<div class="rounded-2xl border border-ink-dim/20 bg-surface px-4 py-3">
				<div class="flex items-baseline justify-between gap-2">
					<a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer"
						class="font-mono text-sm font-semibold text-ink hover:text-azure">OpenStreetMap</a>
					<span class="shrink-0 text-xs text-ink-muted">© OpenStreetMap contributors · ODbL</span>
				</div>
				<p class="mt-0.5 text-xs text-ink-muted">Tile data and cartography used for the node map</p>
			</div>
		</section>

		<p class="text-center text-xs text-ink-muted">
			gomeshcom is not affiliated with or endorsed by the MeshCom project or ÖVSV/ICSSW.
		</p>
	</div>
</div>
