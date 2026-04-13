import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  CirclePlus,
  Cpu,
  LoaderCircle,
  MessageSquareText,
  Radio,
  RefreshCw,
  Search,
  ShieldAlert,
  Signal,
  ToggleLeft,
  ToggleRight,
  Trash2,
  Wifi,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import type { ReactNode } from "react";
import type {
  DiscoveredModem,
  ModemEvent,
  ModemSummary,
  NetworkOption,
  Pagination,
  SmsStorage,
  SmsMessage,
} from "@smsdock/shared";
import { api } from "./api";

type AddFormState = {
  logicalName: string;
  imei: string;
  assignedNetworkMccMnc: string;
  smsReadStorage: SmsStorage;
  pollIntervalSec: number;
  atTimeoutMs: number;
  scanTimeoutSec: number;
  enabled: boolean;
};

const smsStorageOptions: SmsStorage[] = ["SM", "ME", "MT"];

const initialAddForm: AddFormState = {
  logicalName: "",
  imei: "",
  assignedNetworkMccMnc: "",
  smsReadStorage: "SM",
  pollIntervalSec: 10,
  atTimeoutMs: 3000,
  scanTimeoutSec: 90,
  enabled: true,
};

const smsPageSize = 10;
const eventsPageSize = 10;

const emptySmsPagination: Pagination = {
  page: 1,
  pageSize: smsPageSize,
  totalItems: 0,
  totalPages: 1,
};

const emptyEventPagination: Pagination = {
  page: 1,
  pageSize: eventsPageSize,
  totalItems: 0,
  totalPages: 1,
};

function safePagination(value: Pagination | null | undefined, fallback: Pagination): Pagination {
  if (!value) {
    return fallback;
  }

  return {
    page: value.page || fallback.page,
    pageSize: value.pageSize || fallback.pageSize,
    totalItems: value.totalItems || 0,
    totalPages: value.totalPages || 1,
  };
}

export function App() {
  const [modems, setModems] = useState<ModemSummary[]>([]);
  const [selectedModemId, setSelectedModemId] = useState<string | null>(null);
  const [details, setDetails] = useState<ModemSummary | null>(null);
  const [smsItems, setSmsItems] = useState<SmsMessage[]>([]);
  const [smsPagination, setSmsPagination] = useState<Pagination>(emptySmsPagination);
  const [eventsItems, setEventsItems] = useState<ModemEvent[]>([]);
  const [eventsPagination, setEventsPagination] = useState<Pagination>(emptyEventPagination);
  const [smsPage, setSmsPage] = useState(1);
  const [eventsPage, setEventsPage] = useState(1);

  const [isLoadingModems, setIsLoadingModems] = useState(true);
  const [isLoadingDetails, setIsLoadingDetails] = useState(false);
  const [isLoadingSms, setIsLoadingSms] = useState(false);
  const [isLoadingEvents, setIsLoadingEvents] = useState(false);
  const [pageError, setPageError] = useState<string>("");
  const [lastUpdatedAt, setLastUpdatedAt] = useState<string>("");

  const [addDrawerOpen, setAddDrawerOpen] = useState(false);
  const [scanLoading, setScanLoading] = useState(false);
  const [scanError, setScanError] = useState("");
  const [discovered, setDiscovered] = useState<DiscoveredModem[]>([]);
  const [selectedDiscoveryIMEI, setSelectedDiscoveryIMEI] = useState("");
  const [addForm, setAddForm] = useState<AddFormState>(initialAddForm);
  const [createLoading, setCreateLoading] = useState(false);

  const [settingsLoading, setSettingsLoading] = useState(false);
  const [networks, setNetworks] = useState<NetworkOption[]>([]);
  const [networkScanLoading, setNetworkScanLoading] = useState(false);
  const [networkActionError, setNetworkActionError] = useState("");
  const [selectedNetworkCode, setSelectedNetworkCode] = useState("");

  const selectedModem =
    details && details.id === selectedModemId
      ? details
      : modems.find((item) => item.id === selectedModemId) ?? null;

  const resetDataViews = useCallback(() => {
    setDetails(null);
    setSmsItems([]);
    setEventsItems([]);
    setSmsPagination(emptySmsPagination);
    setEventsPagination(emptyEventPagination);
    setNetworks([]);
    setSelectedNetworkCode("");
  }, []);

  const patchSelectedModem = useCallback(
    (patch: Partial<ModemSummary>) => {
      if (!selectedModemId) {
        return;
      }

      setModems((current) =>
        current.map((item) => (item.id === selectedModemId ? { ...item, ...patch } : item)),
      );
      setDetails((current) =>
        current && current.id === selectedModemId ? { ...current, ...patch } : current,
      );
    },
    [selectedModemId],
  );

  const refreshModems = useCallback(async (silent = false) => {
    if (!silent) {
      setIsLoadingModems(true);
    }

    try {
      const response = await api.listModems();
      setModems(response.modems);
      setPageError("");
      setLastUpdatedAt(new Date().toISOString());

      if (response.modems.length === 0) {
        setSelectedModemId(null);
        resetDataViews();
        return;
      }

      setSelectedModemId((current) => {
        if (current && response.modems.some((modem) => modem.id === current)) {
          return current;
        }
        return response.modems[0].id;
      });
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      if (!silent) {
        setIsLoadingModems(false);
      }
    }
  }, [resetDataViews]);

  const refreshDetails = useCallback(async (modemId: string, silent = false) => {
    if (!silent) {
      setIsLoadingDetails(true);
    }

    try {
      const response = await api.getModem(modemId);
      setDetails(response.modem);
      setPageError("");
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      if (!silent) {
        setIsLoadingDetails(false);
      }
    }
  }, []);

  const refreshSms = useCallback(async (modemId: string, page: number, silent = false) => {
    if (!silent) {
      setIsLoadingSms(true);
    }

    try {
      const response = await api.listSms(modemId, page, smsPageSize);
      const items = response.sms ?? [];
      const pagination = safePagination(response.pagination, emptySmsPagination);

      setSmsItems(items);
      setSmsPagination(pagination);
      if (page > pagination.totalPages) {
        setSmsPage(pagination.totalPages);
      }
      setPageError("");
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      if (!silent) {
        setIsLoadingSms(false);
      }
    }
  }, []);

  const refreshEvents = useCallback(async (modemId: string, page: number, silent = false) => {
    if (!silent) {
      setIsLoadingEvents(true);
    }

    try {
      const response = await api.listEvents(modemId, page, eventsPageSize);
      const items = response.events ?? [];
      const pagination = safePagination(response.pagination, emptyEventPagination);

      setEventsItems(items);
      setEventsPagination(pagination);
      if (page > pagination.totalPages) {
        setEventsPage(pagination.totalPages);
      }
      setPageError("");
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      if (!silent) {
        setIsLoadingEvents(false);
      }
    }
  }, []);

  useEffect(() => {
    void refreshModems();
  }, [refreshModems]);

  useEffect(() => {
    if (!selectedModemId) {
      resetDataViews();
      return;
    }

    setSmsPage(1);
    setEventsPage(1);
    setNetworks([]);
    setSelectedNetworkCode("");
    void refreshDetails(selectedModemId);
  }, [refreshDetails, resetDataViews, selectedModemId]);

  useEffect(() => {
    if (!selectedModemId) {
      return;
    }
    void refreshSms(selectedModemId, smsPage);
  }, [refreshSms, selectedModemId, smsPage]);

  useEffect(() => {
    if (!selectedModemId) {
      return;
    }
    void refreshEvents(selectedModemId, eventsPage);
  }, [eventsPage, refreshEvents, selectedModemId]);

  useEffect(() => {
    const interval = window.setInterval(() => {
      if (document.visibilityState === "visible") {
        void refreshModems(true);
      }
    }, 12_000);

    return () => window.clearInterval(interval);
  }, [refreshModems]);

  useEffect(() => {
    if (!selectedModemId) {
      return;
    }

    const interval = window.setInterval(() => {
      if (document.visibilityState === "visible") {
        void refreshDetails(selectedModemId, true);
        void refreshSms(selectedModemId, smsPage, true);
        void refreshEvents(selectedModemId, eventsPage, true);
      }
    }, 10_000);

    return () => window.clearInterval(interval);
  }, [eventsPage, refreshDetails, refreshEvents, refreshSms, selectedModemId, smsPage]);

  function handleOpenAddDrawer() {
    setAddDrawerOpen(true);
    setScanError("");
    setDiscovered([]);
    setSelectedDiscoveryIMEI("");
    setAddForm(initialAddForm);
  }

  async function handleScanModems() {
    setScanLoading(true);
    setScanError("");

    try {
      const response = await api.scanModems();
      setDiscovered(response.modems);

      if (response.modems[0]) {
        const first = response.modems[0];
        setSelectedDiscoveryIMEI(first.imei);
        setAddForm({
          logicalName: `modem-${String(response.modems.length).padStart(2, "0")}`,
          imei: first.imei,
          assignedNetworkMccMnc: first.currentNetworkCode,
          smsReadStorage: "SM",
          pollIntervalSec: 10,
          atTimeoutMs: 3000,
          scanTimeoutSec: 90,
          enabled: true,
        });
      }
    } catch (error) {
      setScanError(getErrorMessage(error));
    } finally {
      setScanLoading(false);
    }
  }

  function handleChooseDiscovered(modem: DiscoveredModem) {
    setSelectedDiscoveryIMEI(modem.imei);
    setAddForm((current) => ({
      ...current,
      imei: modem.imei,
      assignedNetworkMccMnc: modem.currentNetworkCode || current.assignedNetworkMccMnc,
    }));
  }

  async function handleCreateModem() {
    setCreateLoading(true);
    setScanError("");

    try {
      const response = await api.createModem(addForm);
      setAddDrawerOpen(false);
      setSelectedModemId(response.modem.id);
      setDetails(response.modem);
      setSmsPage(1);
      setEventsPage(1);
      await refreshModems(true);
      await refreshSms(response.modem.id, 1);
      await refreshEvents(response.modem.id, 1);
    } catch (error) {
      setScanError(getErrorMessage(error));
    } finally {
      setCreateLoading(false);
    }
  }

  async function handleDeleteModem() {
    if (!selectedModem) {
      return;
    }

    const accepted = window.confirm(
      `Удалить модем "${selectedModem.logicalName}" и связанный архив SMS/событий?`,
    );
    if (!accepted) {
      return;
    }

    setSettingsLoading(true);
    try {
      await api.deleteModem(selectedModem.id);
      resetDataViews();
      await refreshModems(true);
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      setSettingsLoading(false);
    }
  }

  async function handleToggleModem() {
    if (!selectedModem) {
      return;
    }

    setSettingsLoading(true);
    try {
      const response = selectedModem.enabled
        ? await api.disableModem(selectedModem.id)
        : await api.enableModem(selectedModem.id);
      setDetails(response.modem);
      await refreshModems(true);
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      setSettingsLoading(false);
    }
  }

  async function handleSaveSettings() {
    if (!selectedModem) {
      return;
    }

    setSettingsLoading(true);
    try {
      const response = await api.updateModem(selectedModem.id, {
        logicalName: selectedModem.logicalName,
        assignedNetworkMccMnc: selectedModem.assignedNetworkMccMnc,
        smsReadStorage: selectedModem.smsReadStorage,
        pollIntervalSec: selectedModem.pollIntervalSec,
        atTimeoutMs: selectedModem.atTimeoutMs,
        scanTimeoutSec: selectedModem.scanTimeoutSec,
      });
      setDetails(response.modem);
      await refreshModems(true);
    } catch (error) {
      setPageError(getErrorMessage(error));
    } finally {
      setSettingsLoading(false);
    }
  }

  async function handleScanNetworks() {
    if (!selectedModem) {
      return;
    }

    setNetworkScanLoading(true);
    setNetworkActionError("");

    try {
      const response = await api.scanNetworks(selectedModem.id);
      setNetworks(response.networks);
      setSelectedNetworkCode(response.networks[0]?.code ?? "");
    } catch (error) {
      setNetworkActionError(getErrorMessage(error));
    } finally {
      setNetworkScanLoading(false);
    }
  }

  async function handleSelectNetwork() {
    if (!selectedModem || !selectedNetworkCode) {
      return;
    }

    setNetworkScanLoading(true);
    setNetworkActionError("");

    try {
      const response = await api.selectNetwork(selectedModem.id, {
        mccMnc: selectedNetworkCode,
      });
      setDetails(response.modem);
      await refreshModems(true);
    } catch (error) {
      setNetworkActionError(getErrorMessage(error));
    } finally {
      setNetworkScanLoading(false);
    }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand-panel">
          <span className="brand-chip">smsdock</span>
          <h1>Модемный док для SMS и ручной работы с сетью.</h1>
          <p>
            Backend держит polling модемов и recovery, интерфейс показывает живой срез
            состояния без тяжелой realtime-схемы.
          </p>
          <div className="brand-actions">
            <button className="primary-button" onClick={() => void refreshModems()}>
              <RefreshCw size={16} />
              Обновить
            </button>
            <button className="secondary-button" onClick={handleOpenAddDrawer}>
              <CirclePlus size={16} />
              Добавить модем
            </button>
          </div>
          <div className="timestamp">
            Последний снимок: {lastUpdatedAt ? formatDateTime(lastUpdatedAt) : "еще нет данных"}
          </div>
        </div>

        <div className="sidebar-section">
          <div className="section-title">
            <Radio size={16} />
            Модемы
          </div>
          {isLoadingModems ? (
            <div className="empty-state compact">
              <LoaderCircle className="spin" size={18} />
              Загружаю список модемов...
            </div>
          ) : modems.length === 0 ? (
            <div className="empty-state compact">
              <ShieldAlert size={18} />
              Пока нет зарегистрированных модемов.
            </div>
          ) : (
            <div className="modem-list">
              {modems.map((modem) => (
                <button
                  key={modem.id}
                  className={`modem-card ${selectedModemId === modem.id ? "active" : ""}`}
                  onClick={() => setSelectedModemId(modem.id)}
                >
                  <div className="modem-card-top">
                    <div>
                      <strong>{modem.logicalName}</strong>
                      <span>{modem.imei}</span>
                    </div>
                    <span className={`status-badge status-${modem.status}`}>{modem.status}</span>
                  </div>
                  <div className="modem-card-meta">
                    <span>
                      <Signal size={14} />
                      {modem.signalStrength >= 0 ? modem.signalStrength : "нет"}
                    </span>
                    <span>
                      <Wifi size={14} />
                      {modem.currentNetworkName || modem.assignedNetworkMccMnc}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </aside>

      <main className="main-panel">
        {pageError ? (
          <div className="alert-banner">
            <AlertTriangle size={16} />
            {pageError}
          </div>
        ) : null}

        {!selectedModem ? (
          <section className="placeholder-panel">
            <Cpu size={36} />
            <h2>Нет выбранного модема</h2>
            <p>Добавьте устройство через UI или выберите уже зарегистрированный модем.</p>
          </section>
        ) : (
          <>
            <section className="hero-card">
              <div className="hero-copy">
                <span className="eyebrow">операционный срез</span>
                <h2>{selectedModem.logicalName}</h2>
                <p>
                  IMEI {selectedModem.imei}. Закрепленная сеть{" "}
                  <strong>{selectedModem.assignedNetworkMccMnc}</strong>. Последний успешный poll{" "}
                  <strong>
                    {selectedModem.lastSuccessAt
                      ? formatDateTime(selectedModem.lastSuccessAt)
                      : "еще не был успешным"}
                  </strong>
                  .
                </p>
              </div>

              <div className="hero-actions">
                <button className="secondary-button" onClick={() => void refreshDetails(selectedModem.id)}>
                  {isLoadingDetails ? <LoaderCircle className="spin" size={16} /> : <RefreshCw size={16} />}
                  Обновить модем
                </button>
                <button className="secondary-button" onClick={handleToggleModem} disabled={settingsLoading}>
                  {selectedModem.enabled ? <ToggleRight size={16} /> : <ToggleLeft size={16} />}
                  {selectedModem.enabled ? "Отключить" : "Включить"}
                </button>
                <button className="danger-button" onClick={handleDeleteModem} disabled={settingsLoading}>
                  <Trash2 size={16} />
                  Удалить
                </button>
              </div>
            </section>

            <section className="stats-grid">
              <MetricCard
                icon={<Signal size={18} />}
                label="Сигнал"
                value={selectedModem.signalStrength >= 0 ? String(selectedModem.signalStrength) : "нет"}
              />
              <MetricCard
                icon={<Wifi size={18} />}
                label="Текущая сеть"
                value={selectedModem.currentNetworkName || "не выбрана"}
              />
              <MetricCard icon={<Cpu size={18} />} label="SIM" value={selectedModem.simState || "unknown"} />
              <MetricCard
                icon={<RefreshCw size={18} />}
                label="Последний poll"
                value={selectedModem.lastPollAt ? formatDateTime(selectedModem.lastPollAt) : "нет"}
              />
            </section>

            <section className="content-grid">
              <div className="panel-card">
                <div className="panel-header">
                  <div>
                    <span className="eyebrow">настройки</span>
                    <h3>Параметры модема</h3>
                  </div>
                  <button className="primary-button" onClick={handleSaveSettings} disabled={settingsLoading}>
                    {settingsLoading ? <LoaderCircle className="spin" size={16} /> : <RefreshCw size={16} />}
                    Сохранить
                  </button>
                </div>
                <div className="form-grid">
                  <label>
                    Имя
                    <input
                      value={selectedModem.logicalName}
                      onChange={(event) => patchSelectedModem({ logicalName: event.target.value })}
                    />
                  </label>
                  <label>
                    Poll, сек
                    <input
                      type="number"
                      min={5}
                      value={selectedModem.pollIntervalSec}
                      onChange={(event) =>
                        patchSelectedModem({ pollIntervalSec: Number(event.target.value) || 5 })
                      }
                    />
                  </label>
                  <label>
                    Хранилище SMS
                    <select
                      value={selectedModem.smsReadStorage}
                      onChange={(event) =>
                        patchSelectedModem({ smsReadStorage: event.target.value as SmsStorage })
                      }
                    >
                      {smsStorageOptions.map((storage) => (
                        <option key={storage} value={storage}>
                          {storage}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    AT timeout, мс
                    <input
                      type="number"
                      min={500}
                      value={selectedModem.atTimeoutMs}
                      onChange={(event) =>
                        patchSelectedModem({ atTimeoutMs: Number(event.target.value) || 500 })
                      }
                    />
                  </label>
                  <label>
                    Scan timeout, сек
                    <input
                      type="number"
                      min={30}
                      value={selectedModem.scanTimeoutSec}
                      onChange={(event) =>
                        patchSelectedModem({ scanTimeoutSec: Number(event.target.value) || 30 })
                      }
                    />
                  </label>
                </div>
              </div>

              <div className="panel-card">
                <div className="panel-header">
                  <div>
                    <span className="eyebrow">сеть</span>
                    <h3>Ручной перевыбор</h3>
                  </div>
                  <button className="secondary-button" onClick={handleScanNetworks} disabled={networkScanLoading}>
                    {networkScanLoading ? <LoaderCircle className="spin" size={16} /> : <Search size={16} />}
                    Найти сети
                  </button>
                </div>

                <div className="network-lock">
                  Автопереключение отключено. После сбоев backend пытается вернуться только в сеть{" "}
                  <strong>{selectedModem.assignedNetworkMccMnc}</strong>.
                </div>

                {networkActionError ? <div className="alert-inline">{networkActionError}</div> : null}

                <div className="network-list">
                  {networks.length === 0 ? (
                    <div className="empty-state compact">
                      <Wifi size={18} />
                      Список сетей появится после ручного поиска.
                    </div>
                  ) : (
                    networks.map((network) => (
                      <label key={network.code} className="network-option">
                        <input
                          type="radio"
                          name="network"
                          value={network.code}
                          checked={selectedNetworkCode === network.code}
                          onChange={() => setSelectedNetworkCode(network.code)}
                        />
                        <div>
                          <strong>{network.name}</strong>
                          <span>
                            {network.code} · {network.status}
                          </span>
                        </div>
                      </label>
                    ))
                  )}
                </div>

                <button
                  className="primary-button wide"
                  onClick={handleSelectNetwork}
                  disabled={!selectedNetworkCode || networkScanLoading}
                >
                  <Wifi size={16} />
                  Выбрать сеть
                </button>
              </div>
            </section>

            <section className="wide-stack">
              <MessagePanel
                loading={isLoadingSms}
                sms={smsItems}
                pagination={smsPagination}
                onPrevious={() => setSmsPage((current) => Math.max(1, current - 1))}
                onNext={() =>
                  setSmsPage((current) => Math.min(smsPagination.totalPages, current + 1))
                }
              />
              <EventPanel
                loading={isLoadingEvents}
                events={eventsItems}
                pagination={eventsPagination}
                onPrevious={() => setEventsPage((current) => Math.max(1, current - 1))}
                onNext={() =>
                  setEventsPage((current) => Math.min(eventsPagination.totalPages, current + 1))
                }
              />
            </section>
          </>
        )}
      </main>

      {addDrawerOpen ? (
        <div className="drawer-backdrop">
          <div className="drawer">
            <div className="panel-header">
              <div>
                <span className="eyebrow">регистрация модема</span>
                <h3>Добавить устройство</h3>
              </div>
              <button className="ghost-button" onClick={() => setAddDrawerOpen(false)}>
                Закрыть
              </button>
            </div>

            <div className="drawer-actions">
              <button className="secondary-button" onClick={handleScanModems} disabled={scanLoading}>
                {scanLoading ? <LoaderCircle className="spin" size={16} /> : <Search size={16} />}
                Сканировать доступные модемы
              </button>
            </div>

            {scanError ? <div className="alert-inline">{scanError}</div> : null}

            <div className="drawer-grid">
              <div className="discovery-list">
                {discovered.length === 0 ? (
                  <div className="empty-state compact">
                    <Cpu size={18} />
                    После сканирования здесь появятся доступные модемы.
                  </div>
                ) : (
                  discovered.map((item) => (
                    <button
                      key={item.imei}
                      className={`discover-card ${selectedDiscoveryIMEI === item.imei ? "active" : ""}`}
                      onClick={() => handleChooseDiscovered(item)}
                    >
                      <strong>{item.model || item.manufacturer || "modem"}</strong>
                      <span>{item.imei}</span>
                      <span>{item.currentNetworkName || item.currentNetworkCode || "сеть не определена"}</span>
                    </button>
                  ))
                )}
              </div>

              <div className="form-grid">
                <label>
                  Логическое имя
                  <input
                    value={addForm.logicalName}
                    onChange={(event) => setAddForm((current) => ({ ...current, logicalName: event.target.value }))}
                  />
                </label>
                <label>
                  IMEI
                  <input
                    value={addForm.imei}
                    onChange={(event) => setAddForm((current) => ({ ...current, imei: event.target.value }))}
                  />
                </label>
                <label>
                  Закрепленная сеть
                  <input
                    value={addForm.assignedNetworkMccMnc}
                    onChange={(event) =>
                      setAddForm((current) => ({
                        ...current,
                        assignedNetworkMccMnc: event.target.value,
                      }))
                    }
                  />
                </label>
                <label>
                  Хранилище SMS
                  <select
                    value={addForm.smsReadStorage}
                    onChange={(event) =>
                      setAddForm((current) => ({
                        ...current,
                        smsReadStorage: event.target.value as SmsStorage,
                      }))
                    }
                  >
                    {smsStorageOptions.map((storage) => (
                      <option key={storage} value={storage}>
                        {storage}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Poll, сек
                  <input
                    type="number"
                    min={5}
                    value={addForm.pollIntervalSec}
                    onChange={(event) =>
                      setAddForm((current) => ({
                        ...current,
                        pollIntervalSec: Number(event.target.value) || 5,
                      }))
                    }
                  />
                </label>
                <label>
                  AT timeout, мс
                  <input
                    type="number"
                    min={500}
                    value={addForm.atTimeoutMs}
                    onChange={(event) =>
                      setAddForm((current) => ({
                        ...current,
                        atTimeoutMs: Number(event.target.value) || 500,
                      }))
                    }
                  />
                </label>
                <label>
                  Scan timeout, сек
                  <input
                    type="number"
                    min={30}
                    value={addForm.scanTimeoutSec}
                    onChange={(event) =>
                      setAddForm((current) => ({
                        ...current,
                        scanTimeoutSec: Number(event.target.value) || 30,
                      }))
                    }
                  />
                </label>
              </div>
            </div>

            <button className="primary-button wide" onClick={handleCreateModem} disabled={createLoading}>
              {createLoading ? <LoaderCircle className="spin" size={16} /> : <CirclePlus size={16} />}
              Зарегистрировать модем
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function MetricCard({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="metric-card">
      <div className="metric-icon">{icon}</div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function MessagePanel({
  loading,
  sms,
  pagination,
  onPrevious,
  onNext,
}: {
  loading: boolean;
  sms: SmsMessage[];
  pagination: Pagination;
  onPrevious: () => void;
  onNext: () => void;
}) {
  const items = sms ?? [];

  return (
    <div className="panel-card wide">
      <div className="panel-header">
        <div>
          <span className="eyebrow">входящие</span>
          <h3>SMS архив</h3>
        </div>
        <div className="panel-actions">
          {loading ? <LoaderCircle className="spin" size={16} /> : <MessageSquareText size={18} />}
          <PaginationBar pagination={pagination} onPrevious={onPrevious} onNext={onNext} />
        </div>
      </div>

      {items.length === 0 ? (
        <div className="empty-state">
          <MessageSquareText size={24} />
          В этом архиве пока нет SMS.
        </div>
      ) : (
        <div className="sms-list">
          {items.map((message) => (
            <article key={message.id} className="sms-card">
              <header>
                <strong>{message.sender}</strong>
                <span>{formatDateTime(message.receivedAt)}</span>
              </header>
              <p>{message.body}</p>
              <footer>
                <span>{message.encoding}</span>
                <span>
                  {message.storage}
                  {message.storageIndex ? ` #${message.storageIndex}` : ""}
                </span>
                <span>
                  {message.multipartTotal
                    ? `part ${message.multipartPart}/${message.multipartTotal}`
                    : "single"}
                </span>
              </footer>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}

function EventPanel({
  loading,
  events,
  pagination,
  onPrevious,
  onNext,
}: {
  loading: boolean;
  events: ModemEvent[];
  pagination: Pagination;
  onPrevious: () => void;
  onNext: () => void;
}) {
  const items = events ?? [];

  return (
    <div className="panel-card wide">
      <div className="panel-header">
        <div>
          <span className="eyebrow">эксплуатация</span>
          <h3>Журнал событий</h3>
        </div>
        <div className="panel-actions">
          {loading ? <LoaderCircle className="spin" size={16} /> : <ShieldAlert size={18} />}
          <PaginationBar pagination={pagination} onPrevious={onPrevious} onNext={onNext} />
        </div>
      </div>

      {items.length === 0 ? (
        <div className="empty-state">
          <ShieldAlert size={24} />
          События появятся после первых операций и recovery-сценариев.
        </div>
      ) : (
        <div className="event-list">
          {items.map((event) => (
            <article key={event.id} className={`event-row level-${event.level}`}>
              <div>
                <strong>{event.type}</strong>
                <p>{event.message}</p>
              </div>
              <span>{formatDateTime(event.createdAt)}</span>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}

function PaginationBar({
  pagination,
  onPrevious,
  onNext,
}: {
  pagination: Pagination;
  onPrevious: () => void;
  onNext: () => void;
}) {
  return (
    <div className="pagination-bar">
      <span className="pagination-summary">
        стр. {pagination.page}/{pagination.totalPages} · {pagination.totalItems}
      </span>
      <button
        className="icon-button"
        onClick={onPrevious}
        disabled={pagination.page <= 1}
        aria-label="Предыдущая страница"
      >
        <ChevronLeft size={16} />
      </button>
      <button
        className="icon-button"
        onClick={onNext}
        disabled={pagination.page >= pagination.totalPages}
        aria-label="Следующая страница"
      >
        <ChevronRight size={16} />
      </button>
    </div>
  );
}

function formatDateTime(value: string) {
  const date = new Date(value);
  return new Intl.DateTimeFormat("ru-RU", {
    dateStyle: "short",
    timeStyle: "medium",
  }).format(date);
}

function getErrorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  return "Неизвестная ошибка";
}
