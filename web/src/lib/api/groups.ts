export interface MeshcomGroup {
	group: string;
	prefix: string;
	note: string;
	reach?: string;
	flag: string;
	from: number;
	to: number;
}

const PREFIX_FLAGS: Record<string, string> = {
	EU: '🇪🇺',
	US: '🇺🇸',
	LOC: '📍',
	'WW-GE': '🌍',
	'WW-EN': '🌎',
	DACH: '🏔️',
	F: '🇫🇷',
	PA: '🇳🇱',
	ON: '🇧🇪',
	EA: '🇪🇸',
	I: '🇮🇹',
	YO: '🇷🇴',
	HB: '🇨🇭',
	OE: '🇦🇹',
	G: '🇬🇧',
	OZ: '🇩🇰',
	SA: '🇸🇪',
	SP: '🇵🇱',
	DL: '🇩🇪',
	SV: '🇬🇷',
	B: '🇨🇳',
	'9V': '🇸🇬',
	T7: '🇸🇲',
	S5: '🇸🇮'
};

function flagFor(prefix: string): string {
	if (PREFIX_FLAGS[prefix]) return PREFIX_FLAGS[prefix];
	const base = prefix.split(/[\s0-9]/)[0];
	return PREFIX_FLAGS[base] ?? '📻';
}

function parseRange(group: string): [number, number] {
	const clean = group.replace(/\s+/g, '');
	const dashIdx = clean.indexOf('-');
	if (dashIdx <= 0) {
		const n = parseInt(clean, 10);
		return [n, n];
	}
	return [parseInt(clean.slice(0, dashIdx), 10), parseInt(clean.slice(dashIdx + 1), 10)];
}

function g(group: string, prefix: string, note: string, reach?: string): MeshcomGroup {
	const [from, to] = parseRange(group);
	return { group, prefix, note, reach, flag: flagFor(prefix), from, to };
}

export const KNOWN_GROUPS: MeshcomGroup[] = [
	g('2', 'EU', 'Europa', 'Europaweit'),
	g('3', 'US', 'USA', 'US-weit'),
	g('9', 'LOC', 'Local group', 'MeshCom HF cloud only'),
	g('10', 'WW-GE', 'Worldwide German'),
	g('13', 'WW-EN', 'Worldwide English'),
	g('19000', 'F', 'France dép. 19 & 87'),
	g('20', 'DACH', 'D-A-CH', 'Deutschland, Österreich, Schweiz'),
	g('204', 'PA', 'Netherlands'),
	g('206', 'ON', 'Belgium'),
	g('208', 'F', 'France'),
	g('214', 'EA', 'Spain'),
	g('222', 'I', 'Italy'),
	g('22201', 'I', 'Lazio'),
	g('22202', 'I', 'Sardegna'),
	g('22203', 'I', 'Umbria'),
	g('22211', 'I', 'Liguria'),
	g('22213', 'I', "Valle d'Aosta"),
	g('22221', 'I', 'Lombardia'),
	g('22231', 'I', 'Friuli Venezia Giulia'),
	g('22232', 'I', 'Trentino Alto Adige'),
	g('22233', 'I', 'Veneto'),
	g('22241', 'I', 'Emilia Romagna'),
	g('22251', 'I', 'Toscana'),
	g('22261', 'I', 'Abruzzo'),
	g('22262', 'I', 'Marche'),
	g('22271', 'I', 'Puglia'),
	g('22281', 'I', 'Basilicata'),
	g('22282', 'I', 'Calabria'),
	g('22283', 'I', 'Campania'),
	g('22284', 'I', 'Molise'),
	g('22291', 'I', 'Sicilia'),
	g('22299', 'I', 'Meteo/data/sensors'),
	g('226', 'YO', 'Romania'),
	g('228', 'HB', 'Switzerland'),
	g('232', 'OE', 'Austria'),
	g('2321-2329', 'OE', 'OE Bundesländer'),
	g('234', 'G', 'Great Britain'),
	g('238', 'OZ', 'Denmark'),
	g('240', 'SA', 'Sweden'),
	g('260', 'SP', 'Poland'),
	g('262', 'DL', 'Germany'),
	g('26200-26299', 'DL', 'Germany regional', 'Deutschsprachige Meldungen'),
	g('2622', 'DL', 'Schleswig-Holstein'),
	g('26206', 'DL', 'DARC OV Dachau C06'),
	g('26207', 'DL', 'Sachsen-Anhalt'),
	g('26216', 'DL', 'Chiemgau'),
	g('26220', 'DL', 'Großraum Hamburg'),
	g('26221', 'DL', 'Stadt Hamburg'),
	g('26225', 'DL', 'AFU Nord'),
	g('26235', 'DL', 'NI-Südheide'),
	g('26242', 'DL', 'Münsterland'),
	g('26244', 'DL', 'Freising'),
	g('26251', 'DL', 'Rhein-Berg', 'Rheinisch-Bergischer-Kreis'),
	g('26255', 'DL', 'Pfalz'),
	g('26266', 'DL', 'Saar'),
	g('26269', 'DL', 'Hessen/Rheinland Pfalz'),
	g('26289', 'DL', 'München Stadt'),
	g('26295', 'DL', 'Ostthüringen'),
	g('26298', 'DL', 'Thüringen'),
	g('26379', 'DL', 'Hochrhein'),
	g('292', 'T7', 'San Marino'),
	g('293', 'S5', 'Slovenia'),
	g('30', 'SV', 'Greece'),
	g('460', 'B', 'China'),
	g('901', '9V', 'Singapore')
];

export function resolveGroup(dst: string): MeshcomGroup | null {
	const n = parseInt(dst, 10);
	if (isNaN(n)) return null;
	const exact = KNOWN_GROUPS.find((grp) => grp.from === grp.to && grp.from === n);
	if (exact) return exact;
	return KNOWN_GROUPS.find((grp) => grp.from !== grp.to && n >= grp.from && n <= grp.to) ?? null;
}

export function groupTooltip(group: MeshcomGroup): string {
	const parts = [`Group ${group.group}`, group.prefix];
	if (group.reach) parts.push(group.reach);
	return parts.join(' · ');
}

export interface PartitionedChannels {
	known: Array<{ channel: string; group: MeshcomGroup }>;
	unknown: string[];
}

export function partitionChannels(channels: string[]): PartitionedChannels {
	const known: Array<{ channel: string; group: MeshcomGroup }> = [];
	const unknown: string[] = [];
	for (const ch of channels) {
		if (ch === 'Broadcast') continue;
		const group = resolveGroup(ch);
		if (group) known.push({ channel: ch, group });
		else unknown.push(ch);
	}
	known.sort((a, b) => a.group.from - b.group.from);
	return { known, unknown };
}
