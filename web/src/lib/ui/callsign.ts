const IU5PMP_BASE = 'IU5PMP';
const QQ_PREFIX = 'QQ';
const CALLSIGN_PATTERN = /^(?:IU5PMP|QQ[A-Z0-9]{3,8})(?:-[0-9]{1,2})?$/;

export function normalizeCallsign(value: string): string {
	const call = value.trim().toUpperCase();
	if (call === '') return '';

	const [base, suffix] = call.split('-', 2);
	if (base === IU5PMP_BASE || base.startsWith(QQ_PREFIX) || base.length < 2) {
		return call;
	}

	const normalized = QQ_PREFIX + base.slice(2);
	return suffix === undefined ? normalized : `${normalized}-${suffix}`;
}

export function isValidCallsign(value: string): boolean {
	return CALLSIGN_PATTERN.test(normalizeCallsign(value));
}
