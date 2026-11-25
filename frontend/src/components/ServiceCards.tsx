import type { ServiceStatus } from '../types';

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit' });

export function ServiceCards({ services }: { services: ServiceStatus[] }) {
  return (
    <div className="cards-grid">
      {services.map((s) => (
        <div key={s.id} className={`card svc-card ${s.reminder ? 'svc-urgent' : ''}`}>
          <div className="svc-head">
            <span className="svc-name">{s.name}</span>
            {s.reminder && <span className="badge badge-red">срочно</span>}
          </div>
          <span className="label">{s.kind}{s.account_no && ` · л/с ${s.account_no}`}</span>
          <div className="svc-debt">
            {s.debt.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽
          </div>
          <span className={`deadline ${s.reminder ? 'neg' : ''}`}>
            оплатить до {fmtDate(s.next_deadline)} · {s.days_left <= 0 ? 'сегодня' : `${s.days_left} дн.`}
          </span>
        </div>
      ))}
    </div>
  );
}
