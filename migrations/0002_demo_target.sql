-- Migration Safety Engine — Phase 2 demo TARGET table.
--
-- This is the *application* table the engine operates on (NOT an engine control
-- table). It starts in a pre-migration shape: weight/dims plus a `legacy_shipping`
-- column, and deliberately has NO `shipping_class` column yet — the engine adds it
-- (expand), backfills it from weight/dims, verifies, then drops `legacy_shipping`
-- (contract). Seeded with enough rows that batched backfill is observable.
-- Idempotent: safe to run repeatedly.

CREATE TABLE IF NOT EXISTS catalog_product (
    id              bigserial PRIMARY KEY,
    sku             text    NOT NULL,
    weight          numeric NOT NULL,            -- kg
    dims            text    NOT NULL,            -- "LxWxH" in cm
    legacy_shipping text                          -- old free-text shipping tier (contract drops this)
);

-- Seed 50k rows only if the table is empty (keeps `make migrate` idempotent).
INSERT INTO catalog_product (sku, weight, dims, legacy_shipping)
SELECT
    'SKU-' || g,
    round((random() * 50)::numeric, 2),
    (1 + floor(random() * 100))::int || 'x' ||
    (1 + floor(random() * 100))::int || 'x' ||
    (1 + floor(random() * 100))::int,
    'legacy-tier-' || (g % 3)
FROM generate_series(1, 50000) AS g
WHERE NOT EXISTS (SELECT 1 FROM catalog_product);
