import { useCallback, useEffect, useState } from 'react';
import { api } from './api';
import type { MonthStat, Receipt, Summary } from './types';
import { ServiceCards } from './components/ServiceCards';
import { QuickPanel } from './components/QuickPanel';
import { ExpenseChart } from './components/ExpenseChart';
import { Receipts } from './components/Receipts';

const currentMonth = () => new Date().toISOString().slice(0, 7);

export default function App() {
  const [summary, setSummary] = useState<Summary>();
  const [stats, setStats] = useState<MonthStat[]>([]);
  const [receipts, setReceipts] = useState<Receipt[]>([]);
  const [month, setMonth] = useState(currentMonth);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const [s, st, rc] = await Promise.all([
        api.summary(),
        api.monthlyStats(),
        api.receipts(month),
      ]);
      setSummary(s);
      setStats(st);
      setReceipts(rc);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [month]);

  useEffect(() => { load(); }, [load]);

  const reminders = summary?.services.filter((x) => x.reminder) ?? [];

  return (
    <div className="app">
      <h1>Коммуналка</h1>
      <p className="subtitle">Долги, показания и чеки — одним экраном</p>

      {error && <div className="error">Ошибка: {error}</div>}

      {reminders.length > 0 && (
        <div className="banner">
          ⏰ Дедлайн близко:{' '}
          {reminders.map((r) => `${r.name} — ${r.days_left <= 0 ? 'сегодня' : `через ${r.days_left} дн.`}`).join('; ')}
        </div>
      )}

      <section className="card total-card">
        <span className="label">Суммарный долг</span>
        <strong className={summary && summary.total_debt > 0 ? 'neg' : 'pos'}>
          {(summary?.total_debt ?? 0).toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽
        </strong>
      </section>

      <ServiceCards services={summary?.services ?? []} />

      <QuickPanel services={summary?.services ?? []} onRefresh={load} />

      <section className="card">
        <h3 className="sub-heading">Расходы по месяцам (оплаты)</h3>
        <ExpenseChart stats={stats} />
      </section>

      <Receipts
        services={summary?.services ?? []}
        month={month}
        setMonth={setMonth}
        receipts={receipts}
        onChanged={load}
      />
    </div>
  );
}
