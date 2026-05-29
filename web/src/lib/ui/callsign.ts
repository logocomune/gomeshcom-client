const CALLSIGN_PATTERN = /^[A-Z]{1,3}[0-9][A-Z0-9]{1,6}(?:-[0-9]{1,2})?$/;

export function normalizeCallsign(value: string): string {
	return value.trim().toUpperCase();
}

export function isValidCallsign(value: string): boolean {
	return CALLSIGN_PATTERN.test(normalizeCallsign(value));
}
