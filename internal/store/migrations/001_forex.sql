CREATE TABLE IF NOT EXISTS forex_rates (
    date          date        NOT NULL,
    currency_iso3 text        NOT NULL,
    currency_name text        NOT NULL,
    unit          integer     NOT NULL,
    buy           numeric(12,4) NOT NULL,
    sell          numeric(12,4) NOT NULL,
    published_on  timestamptz,
    fetched_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (date, currency_iso3)
);

CREATE INDEX IF NOT EXISTS idx_forex_rates_currency_date
    ON forex_rates (currency_iso3, date DESC);
