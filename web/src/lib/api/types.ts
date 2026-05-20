export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'unauthenticated';

export type MeshcomPacketType = 'msg' | 'pos' | 'tele' | string;

export type MeshcomPacket = {
	type?: MeshcomPacketType;
	src_type?: string;
	src?: string;
	dst?: string;
	msg?: string;
	msg_id?: string;
	lat?: number;
	lat_dir?: string;
	long?: number;
	long_dir?: string;
	aprs_symbol?: string;
	aprs_symbol_group?: string;
	hw_id?: string | number;
	alt?: number;
	batt?: number;
	firmware?: string | number;
	fw_sub?: string;
	rssi?: number;
	snr?: number;
	temp1?: number;
	temp2?: number;
	hum?: number;
	qfe?: number;
	qnh?: number;
	gas?: number;
	co2?: number;
	[key: string]: unknown;
};

export type PacketReceivedPayload = {
	remote_addr?: string;
	received_at?: string;
	packet?: MeshcomPacket;
	replay?: boolean;
};

export type StreamEvent = {
	id: string;
	type: string;
	receivedAt: string;
	data: unknown;
};

export type StationIdentity = {
	callsign: string;
	version?: string;
	txDisabled?: boolean;
	forwardTargetCount?: number;
};

export type PositionRecord = {
	lat: number;
	lng: number;
	alt: number;
	hw_id?: string;
	firstseen: string;
	lastseen: string;
	lastdirectseen?: string;
	rssi: number;
	snr: number;
	via?: string[];
};

export type PositionMap = Record<string, PositionRecord>;

export type ConversationKind = 'broadcast' | 'channel' | 'dm';

export type Conversation = {
	id: string;
	kind: ConversationKind;
	label: string;
	last_seen: string;
	size: number;
};

export type ChatRecordSource = 'event-history' | 'event-live';

export type ChatRecord = {
	received_at: string;
	src?: string;
	src_type?: string;
	dst?: string;
	msg_id?: string;
	msg: string;
	rssi?: number;
	snr?: number;
	source?: ChatRecordSource;
	direction?: 'outbound' | string;
	delivery_status?: 'pending' | 'failed' | string;
};
