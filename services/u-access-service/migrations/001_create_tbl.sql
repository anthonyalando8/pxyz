\c pxyz;

CREATE TABLE countries (
    id SERIAL PRIMARY KEY,
    iso2 CHAR(2) UNIQUE NOT NULL,
    iso3 CHAR(3) UNIQUE NOT NULL,
    name TEXT NOT NULL,
    phone_code TEXT,
    currency_code CHAR(3),
    currency_name TEXT,
    region TEXT,
    subregion TEXT,
    flag_url TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
