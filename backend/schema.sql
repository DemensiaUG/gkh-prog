CREATE TABLE IF NOT EXISTS services (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    kind         TEXT NOT NULL,               -- квартплата, свет, вода, газ, интернет...
    account_no   TEXT NOT NULL DEFAULT '',
    debt         REAL NOT NULL DEFAULT 0,
    deadline_day INTEGER NOT NULL DEFAULT 10, -- день месяца, до которого платить
    has_meter    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS accruals (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    month      TEXT NOT NULL,                 -- YYYY-MM
    amount     REAL NOT NULL CHECK (amount > 0),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS payments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    amount     REAL NOT NULL CHECK (amount > 0),
    paid_on    TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS readings (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    value      REAL NOT NULL,
    period     TEXT NOT NULL,                 -- YYYY-MM
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS receipts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id  INTEGER REFERENCES services(id) ON DELETE SET NULL,
    title       TEXT NOT NULL DEFAULT '',
    month       TEXT NOT NULL,
    filename    TEXT NOT NULL,
    uploaded_at TEXT NOT NULL DEFAULT (datetime('now'))
);
