import { useRef, useState } from 'react';
import { api } from '../api';
import type { Receipt, ServiceStatus } from '../types';

const API_URL = import.meta.env.VITE_API_URL ?? "";

const currentMonth = () => new Date().toISOString().slice(0, 7);

export function Receipts({ services, month, setMonth, receipts, onChanged }: {
  services: ServiceStatus[];
  month: string;
  setMonth: (m: string) => void;
  receipts: Receipt[];
  onChanged: () => void;
}) {
  const [title, setTitle] = useState('');
  const [svcId, setSvcId] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!file) { setError('Выберите файл (PDF или фото)'); return; }
    try {
      const fd = new FormData();
      fd.append('file', file);
      fd.append('title', title.trim());
      fd.append('month', month);
      if (svcId) fd.append('service_id', svcId);
      await api.uploadReceipt(fd);
      setTitle(''); setFile(null);
      if (fileRef.current) fileRef.current.value = '';
      onChanged();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const remove = async (id: number) => {
    await api.removeReceipt(id);
    onChanged();
  };

  return (
    <section className="card">
      <h3 className="sub-heading">Чеки и квитанции</h3>

      <form className="panel-form" onSubmit={submit}>
        <input type="file" ref={fileRef}
          onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
        <input value={title} onChange={(e) => setTitle(e.target.value)}
          placeholder="Название (квитанция за ноябрь)" className="note-input" />
        <input type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
        <select value={svcId} onChange={(e) => setSvcId(e.target.value)}>
          <option value="">Служба (необязательно)</option>
          {services.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
        <button type="submit" className="btn btn-primary">Загрузить</button>
      </form>
      {error && <p className="form-error">{error}</p>}

      {receipts.length === 0 ? (
        <p className="empty">Чеков пока нет</p>
      ) : (
        <ul className="tx-list">
          {receipts.map((r) => (
            <li key={r.id} className="tx">
              <span className="tx-info">
                <span className="tx-title">{r.title || 'Без названия'}</span>
                <span className="tx-meta">
                  {r.month}{r.service && ` · ${r.service}`} · {r.uploaded_at}
                </span>
              </span>
              <a className="btn" href={`${API_URL}/api/receipts/${r.id}/file`} target="_blank"
                rel="noreferrer">Открыть</a>
              <button className="btn btn-danger" onClick={() => remove(r.id)}>✕</button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
