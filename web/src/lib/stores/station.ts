import { writable } from 'svelte/store';
import type { StationIdentity } from '$lib/api/types';

export const stationStore = writable<StationIdentity | null>(null);
