export type ModemStatus =
  | "offline"
  | "binding"
  | "ready"
  | "polling"
  | "scanning"
  | "recovering"
  | "degraded"
  | "disabled";

export type EventLevel = "info" | "warn" | "error";

export type SmsStorage = "SM" | "ME" | "MT";

export interface ModemSummary {
  id: string;
  logicalName: string;
  imei: string;
  assignedNetworkMccMnc: string;
  smsReadStorage: SmsStorage;
  enabled: boolean;
  pollIntervalSec: number;
  atTimeoutMs: number;
  scanTimeoutSec: number;
  status: ModemStatus;
  lastError: string;
  lastSeenAt: string | null;
  currentNetworkMccMnc: string;
  currentNetworkName: string;
  signalStrength: number;
  simState: string;
  lastPollAt: string | null;
  lastSuccessAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface SmsMessage {
  id: string;
  modemId: string;
  storage: SmsStorage;
  storageIndex: number | null;
  sender: string;
  body: string;
  encoding: string;
  rawPdu: string;
  modemTimestamp: string | null;
  receivedAt: string;
  multipartRef: number | null;
  multipartPart: number | null;
  multipartTotal: number | null;
  dedupeKey: string;
}

export interface ModemEvent {
  id: string;
  modemId: string;
  level: EventLevel;
  type: string;
  message: string;
  payloadJson: string;
  createdAt: string;
}

export interface NetworkOption {
  code: string;
  name: string;
  status: string;
}

export interface DiscoveredModem {
  path: string;
  imei: string;
  manufacturer: string;
  model: string;
  firmware: string;
  simState: string;
  iccid: string;
  signalStrength: number;
  currentNetworkCode: string;
  currentNetworkName: string;
}

export interface CreateModemRequest {
  logicalName: string;
  imei: string;
  assignedNetworkMccMnc: string;
  smsReadStorage: SmsStorage;
  pollIntervalSec: number;
  atTimeoutMs: number;
  scanTimeoutSec: number;
  enabled: boolean;
}

export interface UpdateModemRequest {
  logicalName?: string;
  assignedNetworkMccMnc?: string;
  smsReadStorage?: SmsStorage;
  pollIntervalSec?: number;
  atTimeoutMs?: number;
  scanTimeoutSec?: number;
}

export interface SelectNetworkRequest {
  mccMnc: string;
}

export interface ScanModemsResponse {
  modems: DiscoveredModem[];
}

export interface ListModemsResponse {
  modems: ModemSummary[];
}

export interface Pagination {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
}

export interface ModemResponse {
  modem: ModemSummary;
}

export interface ListSmsResponse {
  sms: SmsMessage[];
  pagination: Pagination;
}

export interface ListEventsResponse {
  events: ModemEvent[];
  pagination: Pagination;
}

export interface ScanNetworksResponse {
  modemId: string;
  networks: NetworkOption[];
}

export interface HealthResponse {
  ok: boolean;
  checks: Record<string, string>;
}
