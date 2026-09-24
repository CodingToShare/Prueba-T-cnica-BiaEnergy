-- Source data supplied by the challenge: meters, hourly readings, known events.
-- Timestamps are timezone-naive source wall-clock times (ADR-008).
-- Measurement columns use NUMERIC to persist decimal source values exactly.
-- Only structural checks are enforced here; unusual physical values must stay
-- loadable because they are analytical signals.

-- +goose Up
CREATE TABLE meters (
    meter_id text PRIMARY KEY,
    CONSTRAINT meters_meter_id_not_blank CHECK (btrim(meter_id) <> '')
);

CREATE TABLE readings (
    meter_id          text      NOT NULL REFERENCES meters (meter_id),
    reading_timestamp timestamp NOT NULL,
    consumption_kwh   numeric   NOT NULL,
    voltage_v         numeric   NOT NULL,
    current_a         numeric   NOT NULL,
    power_factor      numeric   NOT NULL,
    source_status     text      NOT NULL,
    CONSTRAINT readings_pkey PRIMARY KEY (meter_id, reading_timestamp),
    CONSTRAINT readings_measurements_finite CHECK (
        consumption_kwh NOT IN ('NaN', 'Infinity', '-Infinity')
        AND voltage_v NOT IN ('NaN', 'Infinity', '-Infinity')
        AND current_a NOT IN ('NaN', 'Infinity', '-Infinity')
        AND power_factor NOT IN ('NaN', 'Infinity', '-Infinity')
    ),
    CONSTRAINT readings_source_status_not_blank CHECK (btrim(source_status) <> '')
);

CREATE TABLE events (
    id              bigint    GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    meter_id        text      NOT NULL REFERENCES meters (meter_id),
    event_timestamp timestamp NOT NULL,
    event_type      text      NOT NULL,
    description     text      NOT NULL,
    CONSTRAINT events_natural_key UNIQUE (meter_id, event_timestamp, event_type, description),
    CONSTRAINT events_event_type_not_blank CHECK (btrim(event_type) <> ''),
    CONSTRAINT events_description_not_blank CHECK (btrim(description) <> '')
);

-- +goose Down
DROP TABLE events;
DROP TABLE readings;
DROP TABLE meters;
