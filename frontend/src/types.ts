export interface Service {
  id: number;
  name: string;
  kind: string;
  account_no: string;
  debt: number;
  deadline_day: number;
  has_meter: boolean;
}

export interface ServiceStatus extends Service {
  next_deadline: string;
  days_left: number;
  reminder: boolean;
}

export interface Reading {
  id: number;
  service_id: number;
  service: string;
  value: number;
  prev_value: number | null;
  consumption: number | null;
  period: string;
  created_at: string;
}

export interface MonthStat {
  month: string;
  total: number;
}

export interface Receipt {
  id: number;
  service_id: number | null;
  service: string;
  title: string;
  month: string;
  uploaded_at: string;
}

export interface Summary {
  total_debt: number;
  services: ServiceStatus[];
}
