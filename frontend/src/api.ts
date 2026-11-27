import type { MonthStat, Receipt, Reading, Summary } from './types';

const BASE = `${import.meta.env.VITE_API_URL ?? ""}/api`;

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error ?? `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

const post = <T>(url: string, body: unknown) =>
  fetch(`${BASE}/${url}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }).then((r) => handle<T>(r));

export const api = {
  summary: () => fetch(`${BASE}/summary`).then((r) => handle<Summary>(r)),

  monthlyStats: () =>
    fetch(`${BASE}/stats/monthly`).then((r) => handle<MonthStat[]>(r)),

  createReading: (body: { service_id: number; value: number; period: string }) =>
    post<Reading>('readings', body),
  readings: (serviceId: number) =>
    fetch(`${BASE}/readings?service_id=${serviceId}`).then((r) => handle<Reading[]>(r)),
  clearReadings: () =>
    fetch(`${BASE}/readings/clear`, { method: 'POST' }).then((r) => handle<void>(r)),

  addAccrual: (body: { service_id: number; month: string; amount: number }) =>
    post<unknown>('accruals', body),

  addPayment: (body: { service_id: number; amount: number; note: string }) =>
    post<unknown>('payments', body),

  receipts: (month?: string) =>
    fetch(`${BASE}/receipts${month ? `?month=${month}` : ''}`).then((r) => handle<Receipt[]>(r)),
  uploadReceipt: (fd: FormData) =>
    fetch(`${BASE}/receipts`, { method: 'POST', body: fd }).then((r) => handle<Receipt>(r)),
  removeReceipt: (id: number) =>
    fetch(`${BASE}/receipts/${id}`, { method: 'DELETE' }).then((r) => handle<void>(r)),
};
