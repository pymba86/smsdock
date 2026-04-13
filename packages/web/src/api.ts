import type {
  CreateModemRequest,
  ListEventsResponse,
  ListModemsResponse,
  ListSmsResponse,
  ModemResponse,
  ScanModemsResponse,
  ScanNetworksResponse,
  SelectNetworkRequest,
  UpdateModemRequest,
} from "@smsdock/shared";

const apiBase = import.meta.env.VITE_API_BASE ?? "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Request failed with ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export const api = {
  listModems() {
    return request<ListModemsResponse>("/api/modems");
  },
  getModem(id: string) {
    return request<ModemResponse>(`/api/modems/${id}`);
  },
  listSms(id: string, page = 1, pageSize = 20) {
    return request<ListSmsResponse>(`/api/modems/${id}/sms?page=${page}&pageSize=${pageSize}`);
  },
  listEvents(id: string, page = 1, pageSize = 20) {
    return request<ListEventsResponse>(`/api/modems/${id}/events?page=${page}&pageSize=${pageSize}`);
  },
  scanModems() {
    return request<ScanModemsResponse>("/api/modems/scan", {
      method: "POST",
    });
  },
  createModem(payload: CreateModemRequest) {
    return request<ModemResponse>("/api/modems", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },
  deleteModem(id: string) {
    return request<void>(`/api/modems/${id}`, {
      method: "DELETE",
    });
  },
  updateModem(id: string, payload: UpdateModemRequest) {
    return request<ModemResponse>(`/api/modems/${id}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    });
  },
  enableModem(id: string) {
    return request<ModemResponse>(`/api/modems/${id}/enable`, {
      method: "POST",
    });
  },
  disableModem(id: string) {
    return request<ModemResponse>(`/api/modems/${id}/disable`, {
      method: "POST",
    });
  },
  scanNetworks(id: string) {
    return request<ScanNetworksResponse>(`/api/modems/${id}/networks/scan`, {
      method: "POST",
    });
  },
  selectNetwork(id: string, payload: SelectNetworkRequest) {
    return request<ModemResponse>(`/api/modems/${id}/networks/select`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },
};
