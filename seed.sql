-- Seed de La Michi POS — datos de demo. Idempotente (ON CONFLICT DO NOTHING).
-- UUIDs fijos para poder enlazar relaciones de forma determinista.
-- Password de los 3 usuarios: "michi123" (hash bcrypt, portable Go↔Node).

-- ─── Sucursales ──────────────────────────────────────────────────────
INSERT INTO branches (id, name, address, phone) VALUES
  ('11111111-1111-1111-1111-111111111111', 'La Michi Centro', 'Av. Juárez 100, Centro', '5512345678'),
  ('22222222-2222-2222-2222-222222222222', 'La Michi Norte',  'Blvd. Norte 500, Industrial', '5587654321')
ON CONFLICT (id) DO NOTHING;

-- ─── Usuarios ────────────────────────────────────────────────────────
-- owner: sin sucursal (acceso global). manager/employee: en Centro.
INSERT INTO users (id, name, email, password_hash, role, branch_id) VALUES
  ('40fda9be-f1a0-41fb-b5bc-790cd3fad6df', 'César (Dueño)',     'dueno@lamichi.com',     '$2b$10$3V1GLcAgLdKy9PFfTiQoh.2hurpMt2oH5Wo3hsuxr20hHK6Kbi6jS', 'owner',    NULL),
  ('a1111111-0000-0000-0000-000000000001', 'Ana (Encargada)',  'encargado@lamichi.com', '$2b$10$3V1GLcAgLdKy9PFfTiQoh.2hurpMt2oH5Wo3hsuxr20hHK6Kbi6jS', 'manager',  '11111111-1111-1111-1111-111111111111'),
  ('a1111111-0000-0000-0000-000000000002', 'Beto (Empleado)',  'empleado@lamichi.com',  '$2b$10$3V1GLcAgLdKy9PFfTiQoh.2hurpMt2oH5Wo3hsuxr20hHK6Kbi6jS', 'employee', '11111111-1111-1111-1111-111111111111')
ON CONFLICT (email) DO NOTHING;

-- ─── Productos ───────────────────────────────────────────────────────
INSERT INTO products (id, name, category, price) VALUES
  ('c0000000-0000-0000-0000-000000000001', 'Paleta de fresa',            'paleta',      25.00),
  ('c0000000-0000-0000-0000-000000000002', 'Paleta de mango con chile',  'paleta',      28.00),
  ('c0000000-0000-0000-0000-000000000003', 'Nieve de limón',             'nieve',       30.00),
  ('c0000000-0000-0000-0000-000000000004', 'Nieve de chocolate',         'nieve',       32.00),
  ('c0000000-0000-0000-0000-000000000005', 'Agua de horchata',           'agua_fresca', 22.00),
  ('c0000000-0000-0000-0000-000000000006', 'Agua de jamaica',            'agua_fresca', 22.00),
  ('c0000000-0000-0000-0000-000000000007', 'Paleta de coco',             'paleta',      26.00)
ON CONFLICT (id) DO NOTHING;

-- ─── Inventario (7 productos × 2 sucursales = 14 filas) ──────────────
-- En Centro: Paleta de fresa y Nieve de limón quedan en stock CRÍTICO
-- (current_stock <= min_stock) para que el reporte de stock crítico tenga datos.
INSERT INTO inventory (product_id, branch_id, current_stock, min_stock) VALUES
  -- Centro
  ('c0000000-0000-0000-0000-000000000001', '11111111-1111-1111-1111-111111111111',  4, 5),  -- CRÍTICO
  ('c0000000-0000-0000-0000-000000000002', '11111111-1111-1111-1111-111111111111', 40, 5),
  ('c0000000-0000-0000-0000-000000000003', '11111111-1111-1111-1111-111111111111',  4, 5),  -- CRÍTICO
  ('c0000000-0000-0000-0000-000000000004', '11111111-1111-1111-1111-111111111111', 35, 5),
  ('c0000000-0000-0000-0000-000000000005', '11111111-1111-1111-1111-111111111111', 50, 5),
  ('c0000000-0000-0000-0000-000000000006', '11111111-1111-1111-1111-111111111111', 50, 5),
  ('c0000000-0000-0000-0000-000000000007', '11111111-1111-1111-1111-111111111111', 20, 5),
  -- Norte
  ('c0000000-0000-0000-0000-000000000001', '22222222-2222-2222-2222-222222222222', 30, 5),
  ('c0000000-0000-0000-0000-000000000002', '22222222-2222-2222-2222-222222222222', 30, 5),
  ('c0000000-0000-0000-0000-000000000003', '22222222-2222-2222-2222-222222222222', 25, 5),
  ('c0000000-0000-0000-0000-000000000004', '22222222-2222-2222-2222-222222222222', 25, 5),
  ('c0000000-0000-0000-0000-000000000005', '22222222-2222-2222-2222-222222222222', 45, 5),
  ('c0000000-0000-0000-0000-000000000006', '22222222-2222-2222-2222-222222222222', 45, 5),
  ('c0000000-0000-0000-0000-000000000007', '22222222-2222-2222-2222-222222222222', 18, 5)
ON CONFLICT (product_id, branch_id) DO NOTHING;
