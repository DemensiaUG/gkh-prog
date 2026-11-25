import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { Reading, ServiceStatus } from '../types';

type Tab = 'readings' | 'accrual' | 'payment';

const currentMonth = () => new Date().toISOString().slice(0, 7);

export function QuickPanel({ services, onRefresh }: {
  services: ServiceStatus[];
  onRefresh: () => void;
}) {
  const [tab, setTab] = useState<Tab>('readings');
  const [msg, setMsg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // показания
  const meterServices = services.filter((s) => s.has_meter);
  const [svcId, setSvcId] = useState('');
  const [value, setValue] = useState('');
  const [readings, setReadings] = useState<Reading[]>([]);

  // начисление / оплата
  const [opService, setOpService] = useState('');
  const [amount, setAmount] = useState('');
  const [note, setNote] = useState('');

  const loadReadings = useCallback(async () => {
    if (!svcId) { setReadings([]); return; }
    setReadings(await api.readings(Number(svcId)));
  }, [svcId]);

  useEffect(() => { loadReadings().catch(() => {}); }, [loadReadings]);

  const submitReading = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null); setMsg(null);
    const v = parseFloat(value.replace(',', '.'));
    if (!svcId || !v) { setError('Выберите счётчик и введите значение'); return; }
    try {
      const r = await api.createReading({
        service_id: Number(svcId),
        value: v,
        period: currentMonth(),
      });
      setValue('');
      await loadReadings();
      onRefresh();
      setMsg(r.consumption !== null
        ? `Сохранено. Расход: ${r.consumption}`
        : 'Сохранено. Это первое показание — расход посчитаем со следующего раза');
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const clearAllReadings = async () => {
    if (!window.confirm(
      'Удалить все показания всех счётчиков? Историю расхода восстановить нельзя.'
    )) return;
    setError(null); setMsg(null);
    try {
      await api.clearReadings();
      setReadings([]);
      onRefresh();
      setMsg('Все показания удалены. Новое показание станет «первым» — расход начнёт считаться со второго ввода.');
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const submitOp = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null); setMsg(null);
    const v = parseFloat(amount.replace(',', '.'));
    if (!opService || !v || v <= 0) { setError('Выберите службу и введите сумму'); return; }
    try {
      if (tab === 'accrual') {
        await api.addAccrual({ service_id: Number(opService), month: currentMonth(), amount: v });
        setMsg('Начисление добавлено, долг увеличен');
      } else {
        await api.addPayment({ service_id: Number(opService), amount: v, note: note.trim() });
        setMsg('Оплата учтена, долг уменьшен');
      }
      setAmount(''); setNote('');
      onRefresh();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <section className="card panel">
      <div className="tabs">
        {([['readings', 'Показания'], ['accrual', 'Начисление'], ['payment', 'Оплата']] as const)
          .map(([v, label]) => (
            <button key={v} className={`chip ${tab === v ? 'active' : ''}`}
              onClick={() => { setTab(v); setMsg(null); setError(null); }}>{label}</button>
          ))}
      </div>

      {tab === 'readings' && (
        <>
          <form className="panel-form" onSubmit={submitReading}>
            <select value={svcId} onChange={(e) => setSvcId(e.target.value)}>
              <option value="">Счётчик...</option>
              {meterServices.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
            <input inputMode="decimal" value={value} onChange={(e) => setValue(e.target.value)}
              placeholder="Значение счётчика" className="amount-input" />
            <button type="submit" className="btn btn-primary">Передать</button>
          </form>

          <button className="btn btn-ghost-danger readings-clear"
            onClick={clearAllReadings}>
            Очистить все показания
          </button>
        </>
      )}

      {(tab === 'accrual' || tab === 'payment') && (
        <form className="panel-form" onSubmit={submitOp}>
          <select value={opService} onChange={(e) => setOpService(e.target.value)}>
            <option value="">Служба...</option>
            {services.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
          <input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)}
            placeholder="Сумма, ₽" className="amount-input" />
          {tab === 'payment' && (
            <input value={note} onChange={(e) => setNote(e.target.value)}
              placeholder="Комментарий" className="note-input" />
          )}
          <button type="submit" className="btn btn-primary">
            {tab === 'accrual' ? 'Начислить' : 'Оплатить'}
          </button>
        </form>
      )}

      {error && <p className="form-error">{error}</p>}
      {msg && <p className="form-ok">{msg}</p>}

      {tab === 'readings' && readings.length > 0 && (
        <ul className="readings-list">
          {readings.map((r) => (
            <li key={r.id}>
              {r.period}: {r.value}
              {r.consumption !== null && (
                <span className="pos"> · расход {r.consumption}</span>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
