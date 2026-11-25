import type { MonthStat } from '../types';

export function ExpenseChart({ stats }: { stats: MonthStat[] }) {
  const W = 560, H = 190, pad = 34, bw = 56;
  const max = Math.max(...stats.map((s) => s.total), 1);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="chart">
      {stats.map((s, i) => {
        const h = (s.total / max) * (H - pad * 2);
        const x = pad + i * ((W - pad * 2) / stats.length) + 8;
        const label = new Date(s.month + '-01')
          .toLocaleDateString('ru-RU', { month: 'short' });
        return (
          <g key={s.month}>
            <rect x={x} y={H - pad - h} width={bw} height={h || 1}
              rx="6" fill="#6c8cff" opacity="0.85" />
            <text x={x + bw / 2} y={H - pad + 16} textAnchor="middle"
              fontSize="11" fill="#9aa1b5">{label}</text>
            <text x={x + bw / 2} y={H - pad - h - 6} textAnchor="middle"
              fontSize="10" fill="#e8eaf2">
              {s.total > 0 ? `${Math.round(s.total / 1000)}к` : ''}
            </text>
          </g>
        );
      })}
    </svg>
  );
}
