export class UnauthorizedError extends Error {
	constructor(message = 'Unauthorized') {
		super(message);
		this.name = 'UnauthorizedError';
	}
}

type UnauthorizedListener = () => void;
type SessionStatus = {
	required: boolean;
	authenticated: boolean;
};

const unauthorizedListeners = new Set<UnauthorizedListener>();

export function onUnauthorized(listener: UnauthorizedListener): () => void {
	unauthorizedListeners.add(listener);
	return () => unauthorizedListeners.delete(listener);
}

function notifyUnauthorized() {
	for (const listener of unauthorizedListeners) {
		listener();
	}
}

export async function apiFetch(
	input: RequestInfo | URL,
	init: RequestInit = {}
): Promise<Response> {
	const response = await fetch(input, {
		...init,
		credentials: init.credentials ?? 'same-origin'
	});

	if (response.status === 401) {
		notifyUnauthorized();
		throw new UnauthorizedError();
	}

	return response;
}

export async function login(username: string, password: string): Promise<void> {
	const response = await fetch('/api/session', {
		method: 'POST',
		credentials: 'same-origin',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ username, password })
	});

	if (response.status === 401) {
		throw new UnauthorizedError();
	}
	if (!response.ok) {
		throw new Error('Login failed');
	}
}

export async function logout(): Promise<void> {
	const response = await fetch('/api/session', {
		method: 'DELETE',
		credentials: 'same-origin'
	});
	if (!response.ok) {
		throw new Error('Logout failed');
	}
}

export async function getSessionStatus(): Promise<SessionStatus> {
	const response = await fetch('/api/session', {
		method: 'GET',
		credentials: 'same-origin'
	});

	if (!response.ok && response.status !== 401) {
		throw new Error('Session status failed');
	}

	return (await response.json()) as SessionStatus;
}
