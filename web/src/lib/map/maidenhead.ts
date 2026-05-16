const fields = 'ABCDEFGHIJKLMNOPQR';
const subsquares = 'abcdefghijklmnopqrstuvwx';

export function getLocator(lon: number, lat: number, precision = 3): string {
	const safePrecision = Math.max(1, Math.min(5, precision));
	let x = normalizeLongitude(lon) + 180;
	let y = clampLatitude(lat) + 90;

	let locator = fields[Math.floor(x / 20)] + fields[Math.floor(y / 10)];
	if (safePrecision === 1) return locator;

	locator += `${Math.floor((x % 20) / 2)}${Math.floor(y % 10)}`;
	if (safePrecision === 2) return locator;

	x %= 2;
	y %= 1;
	locator += subsquares[Math.floor(x / (2 / 24))] + subsquares[Math.floor(y / (1 / 24))];
	if (safePrecision === 3) return locator;

	x %= 2 / 24;
	y %= 1 / 24;
	locator += `${Math.floor(x / (2 / 240))}${Math.floor(y / (1 / 240))}`;
	if (safePrecision === 4) return locator;

	x %= 2 / 240;
	y %= 1 / 240;
	return locator + subsquares[Math.floor(x / (2 / 5760))] + subsquares[Math.floor(y / (1 / 5760))];
}

function normalizeLongitude(lon: number): number {
	let normalized = lon;
	while (normalized < -180) normalized += 360;
	while (normalized >= 180) normalized -= 360;
	return normalized;
}

function clampLatitude(lat: number): number {
	return Math.max(-90, Math.min(89.999999, lat));
}
