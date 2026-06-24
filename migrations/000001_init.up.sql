-- La Michi POS — schema inicial (traducido del schema Prisma del front).
-- Convención: tablas snake_case plural, columnas snake_case, enums de dominio en español.

-- gen_random_uuid() vive en pgcrypto (incluido en Postgres 13+).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ─── Enums ───────────────────────────────────────────────────────────
CREATE TYPE user_role        AS ENUM ('owner', 'manager', 'employee');
CREATE TYPE product_category AS ENUM ('paleta', 'nieve', 'agua_fresca', 'otro');
CREATE TYPE payment_method   AS ENUM ('cash', 'card', 'transfer');
CREATE TYPE sale_status      AS ENUM ('completed', 'cancelled');

-- ─── branches ────────────────────────────────────────────────────────
CREATE TABLE branches (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT        NOT NULL,
  address    TEXT,
  phone      TEXT,
  active     BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── users ───────────────────────────────────────────────────────────
-- branch_id NULL = owner / acceso global. onDelete: Restrict.
CREATE TABLE users (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  name          TEXT        NOT NULL,
  email         TEXT        NOT NULL UNIQUE,
  password_hash TEXT        NOT NULL,
  role          user_role   NOT NULL DEFAULT 'employee',
  branch_id     UUID        REFERENCES branches(id) ON DELETE RESTRICT,
  active        BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_branch_id ON users(branch_id);

-- ─── products ────────────────────────────────────────────────────────
CREATE TABLE products (
  id         UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT             NOT NULL,
  category   product_category NOT NULL DEFAULT 'paleta',
  price      NUMERIC(10,2)    NOT NULL,
  active     BOOLEAN          NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
  CONSTRAINT chk_products_price_nonneg CHECK (price >= 0)
);

-- ─── inventory ───────────────────────────────────────────────────────
-- Un renglón por (producto, sucursal). onDelete: Restrict.
CREATE TABLE inventory (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id    UUID        NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  branch_id     UUID        NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
  current_stock INT         NOT NULL DEFAULT 0,
  min_stock     INT         NOT NULL DEFAULT 0,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_inventory_product_branch UNIQUE (product_id, branch_id),
  CONSTRAINT chk_inventory_current_nonneg CHECK (current_stock >= 0),
  CONSTRAINT chk_inventory_min_nonneg     CHECK (min_stock >= 0)
);

-- ─── sales ───────────────────────────────────────────────────────────
CREATE TABLE sales (
  id             UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
  branch_id      UUID           NOT NULL REFERENCES branches(id) ON DELETE RESTRICT,
  user_id        UUID           NOT NULL REFERENCES users(id)    ON DELETE RESTRICT,
  total          NUMERIC(10,2)  NOT NULL,
  payment_method payment_method NOT NULL DEFAULT 'cash',
  status         sale_status    NOT NULL DEFAULT 'completed',
  created_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
  CONSTRAINT chk_sales_total_nonneg CHECK (total >= 0)
);
CREATE INDEX idx_sales_branch_created ON sales(branch_id, created_at);

-- ─── sale_items ──────────────────────────────────────────────────────
-- product_name / unit_price = snapshot al momento de la venta.
-- onDelete sale: Cascade; product: Restrict.
CREATE TABLE sale_items (
  id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  sale_id      UUID          NOT NULL REFERENCES sales(id)    ON DELETE CASCADE,
  product_id   UUID          NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  product_name TEXT          NOT NULL,
  unit_price   NUMERIC(10,2) NOT NULL,
  quantity     INT           NOT NULL,
  subtotal     NUMERIC(10,2) NOT NULL,
  CONSTRAINT chk_sale_items_qty_pos       CHECK (quantity > 0),
  CONSTRAINT chk_sale_items_unit_nonneg   CHECK (unit_price >= 0),
  CONSTRAINT chk_sale_items_subtotal_nonneg CHECK (subtotal >= 0)
);
CREATE INDEX idx_sale_items_sale_id    ON sale_items(sale_id);
CREATE INDEX idx_sale_items_product_id ON sale_items(product_id);
