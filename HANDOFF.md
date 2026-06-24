# La Michi POS — Contexto de continuación (handoff)

> Documento para que otra sesión de Claude (o cualquier dev) entienda el estado del
> proyecto y qué sigue. Última actualización: 2026-06-21 (Lambdas ✅ · Rewire front ✅).

## 1. Qué es esto

Reto técnico para un puesto **Fullstack** (empresa RedEnergy; entregar a
jordi.mas@redenergy.mx). Plazo ~1 semana, esfuerzo ~8–12 h. Es un **Punto de Venta
(POS)** para una paletería mexicana con varias sucursales ("La Michi").

**Entregables:** repo GitHub · app desplegada en AWS (URL + acceso consola) ·
diagrama de arquitectura AWS · ERD · PPT 8–10 slides. (Recordar apagar recursos AWS
tras la revisión.)

**Requisitos del reto:** (1) auth con al menos un rol · (2) CRUD de entidades
principales · (3) al menos una relación entre entidades · (4) listado con filtros ·
(5) vista de resumen/reporte (ventas del día, top sabores, stock crítico) ·
(6) persistencia en BD relacional.

## 2. Arquitectura (DECISIÓN IMPORTANTE: front y back SEPARADOS)

Se descartó un monolito Next.js. El usuario eligió separar para mostrar skills de
backend + escalabilidad + costos (escala a cero).

```
FRONT (Next.js)            BACK (Go + Lambda)              DATOS
┌──────────────┐   fetch   ┌────────────────────┐         ┌──────────┐
│ la-michi-pos │ ────────▶ │ la-michi-pos-api   │ ──────▶ │ Postgres │
│ UI + cookie  │  + JWT    │ 4 Lambdas (chi)    │  pgx    │  (RDS)   │
└──────────────┘           └────────────────────┘         └──────────┘
   S3+CloudFront            API Gateway + Lambda            RDS
```

- **FRONT** `~/Documents/Personal_Proyects/la-michi-pos` — Next.js 16 + React 19 +
  TS + Tailwind v4. Solo UI; consume el API por `fetch` + JWT en cookie httpOnly.
- **BACK** `~/Documents/Personal_Proyects/la-michi-pos-api` — **Go + AWS Lambda +
  SAM + Docker (imagen ECR)**. Patrón calcado de `~/Documents/Personal_Proyects/tasktribe`
  (Go+SAM+Docker, `CMD_PATH` build-arg, base `public.ecr.aws/lambda/provided:al2023-arm64`, arm64).
- **Granularidad: 4 Lambdas POR RECURSO** — `cmd/auth`, `cmd/products`, `cmd/sales`,
  `cmd/reports`. Cada una es un binario Go con **router `chi` interno**.
- **Capa de datos: `sqlc`** (SQL → Go type-safe) sobre **pgx**. Migraciones con
  **golang-migrate**.
- Hubo un intento previo en **NestJS**: descartado, respaldado en
  `la-michi-pos-api-nest-backup/` (NO usar).

## 3. Estado actual — HECHO ✅

### Backend Go (`la-michi-pos-api`)
- Estructura: `cmd/{auth,products,sales,reports}` + `internal/{config,database,token,web,authapi,db}`.
- **`docker-compose.yml`**: Postgres 16 en **puerto 5433** (contenedor `lamichi-api-db`,
  vol `lamichi_api_pgdata`). El back es dueño de su propia BD.
- **Migración** `migrations/000001_init.{up,down}.sql`: 6 tablas (branches, users,
  products, inventory, sales, sale_items) + 4 enums + CHECK constraints. Traducida
  del schema Prisma del front.
- **`seed.sql`** idempotente: 2 sucursales, 3 usuarios (pass `michi123`), 7 productos,
  14 inventarios (2 en stock crítico en Centro). UUIDs FIJOS (ej. branch Centro =
  `11111111-1111-1111-1111-111111111111`).
- **sqlc** (`sqlc.yaml`): uuid→string, timestamptz→time.Time, numeric→string.
  Queries de users en `internal/db/queries/users.sql`. Genera `internal/db/*.go`.
- Paquetes compartidos:
  - `config` — carga `DATABASE_URL`, `JWT_SECRET` del entorno.
  - `database` — pool pgx (`MaxConns 4`, ping al iniciar).
  - `token` — JWT HS256 (golang-jwt v5). Claims: `{sub, name, email, role, branchId}`, exp 8h.
  - `web` — helpers JSON + middleware `Authenticator` (Bearer) + `RequireRole` (los "guards").
- **Lambda `auth`** (`internal/authapi` service+handler): `POST /auth/login`,
  `GET /auth/me`. bcrypt para validar password.
- `cmd/auth/main.go`: corre como **Lambda** (chiadapter `NewV2`/`ProxyWithContextV2`)
  o como **HTTP local** según exista `AWS_LAMBDA_RUNTIME_API`.
- **`Dockerfile`** (multi-stage Go 1.26-alpine → base Lambda arm64, `CMD_PATH` build-arg).
- **`template.yaml`** (SAM): `AuthFunction` PackageType Image, HttpApi `/auth/{proxy+}`,
  params `DatabaseUrl`/`JwtSecret`.
- **PROBADO:** `go test ./...` (token + authapi verde), `go run` local, `sam build` OK,
  `sam local start-api` → login 200 vía API GW emulado (DB con `host.docker.internal:5433`).

### Front (`la-michi-pos`) — slice de AUTH cableado al back
- Se quitó NextAuth del flujo. Ahora:
  - `src/actions/auth.actions.ts` — `authenticate()` hace fetch a `${API_URL}/auth/login`,
    guarda el JWT en **cookie httpOnly** `token`, redirige. `logout()` borra la cookie.
  - `src/lib/jwt.ts` — `verifyToken()` con **jose** + `JWT_SECRET` (mismo secreto que el back).
    Define `TOKEN_COOKIE`, tipo `SessionUser`, `UserRole`.
  - `src/lib/session.ts` — `getSession()` / `getToken()` leen la cookie.
  - `src/lib/auth-guards.ts` — `requireAuth()` / `requireRole()` ahora leen la cookie
    (firma intacta → las páginas no cambian).
  - `src/proxy.ts` — middleware propio (Edge): verifica la cookie con jose, protege rutas.
  - `src/app/page.tsx` — usa `requireAuth()` en vez de `auth()`.
- `.env` del front: agregado `API_URL=http://localhost:4000` y `JWT_SECRET` (igual al back).
- **tsc limpio. Login CONFIRMADO funcionando en el navegador** (2026-06-21) — el flujo
  completo front→back→cookie→dashboard quedó verificado de punta a punta.

## 4. Cómo correr en local

```bash
# 1. BD del back (Postgres 16 en 5433)
cd ~/Documents/Personal_Proyects/la-michi-pos-api
docker compose up -d
# (primera vez) migrar + seed:
migrate -path migrations -database "postgres://lamichi:lamichi_dev@localhost:5433/lamichi_pos?sslmode=disable" up
docker exec -i lamichi-api-db psql -U lamichi -d lamichi_pos < seed.sql

# 2. Back auth (HTTP local en :4000)
PORT=4000 go run ./cmd/auth
#   o como Lambda contenedor real:  sam build && sam local start-api --env-vars env.json --port 4001

# 3. Front
cd ~/Documents/Personal_Proyects/la-michi-pos
npm run dev   # el usuario lo corre; login con dueno@lamichi.com / michi123
```

Usuarios seed (pass `michi123`): `dueno@lamichi.com` (owner, sin sucursal) ·
`encargado@lamichi.com` (manager, Centro) · `empleado@lamichi.com` (employee, Centro).

## 5. Qué sigue — PENDIENTE ⏳ (en orden)

1. ~~Confirmar login en navegador~~ ✅ HECHO (2026-06-21) — funciona end-to-end.
2. ~~**Lambda `products`**~~ ✅ HECHO (2026-06-21) — CRUD REST completo. Price expuesto como
   `float64` en JSON. Tests unitarios (11) en verde. `cmd/products` en :4001 local.
   Rutas: `GET /products`, `GET /products/{id}`, `POST` (owner+manager), `PATCH` (owner+manager),
   `DELETE` (owner, soft-delete). Agregado a `template.yaml` como `ProductsFunction`.
   **El front todavía lee de Prisma** — eso se wire en el paso 5 (rewire completo).
   ← SIGUIENTE: Lambda `sales`
3. ~~**Lambda `sales`**~~ ✅ HECHO (2026-06-21) — transacción atómica: sale + sale_items +
   DecrementInventory (WHERE current_stock >= qty para evitar negativos). Precio siempre
   del servidor. `TxRunner` exportado para inyección real (pool) vs mock (tests).
   Tests (11) en verde. `cmd/sales` en :4002 local. Agregado a `template.yaml`.
   Rutas: `POST /sales` (cualquier rol), `GET /sales` (managers ven su sucursal, owner ve todo),
   `GET /sales/{id}`. ← SIGUIENTE: Lambda `reports`
4. ~~**Lambda `reports`**~~ ✅ HECHO (2026-06-21) — 3 endpoints: `GET /reports/daily?date=YYYY-MM-DD`,
   `GET /reports/top-products?days=7&limit=10`, `GET /reports/critical-stock`. Managers ven solo
   su sucursal (del JWT); owners pueden pasar ?branch_id o ver todo. Tests (7) en verde.
   `cmd/reports` en :4003 local. Agregado a `template.yaml`.
   ← SIGUIENTE: Rewire completo del front (paso 5)
5. ~~**Rewire completo del front**~~ ✅ HECHO (2026-06-21)
   - `src/types/api.ts` — tipos locales (ProductCategory, PaymentMethod, UserRole)
   - `src/lib/apiClient.ts` — fetch wrapper con `Authorization: Bearer <cookie>`
   - `src/services/productsService.ts` — llama `/products` y `/products/sellable`
   - `src/services/salesService.ts` — solo tipos; lógica en el back Go
   - `src/actions/products.actions.ts` — CRUD via apiClient
   - `src/actions/sales.actions.ts` — POST /sales via apiClient
   - `src/app/page.tsx` — branch name via `/branches/{id}`
   - `src/app/sales/new/page.tsx` — lista sucursales via `/branches`
   - constants/hooks/components — imports @prisma/client → @/types/api
   - Back: branchesapi handler + /products/sellable + /branches event en SAM
   - **TODO manual**: `rm src/lib/prisma.ts src/types/next-auth.d.ts` (operación destructiva)
   - **TODO manual**: `npm remove @prisma/client prisma next-auth bcryptjs` del package.json
   - Para correr todo localmente: `sam build && sam local start-api --env-vars env.json --port 3000`
     y luego `npm run dev` en el front. API_URL=http://127.0.0.1:3000
   - **VERIFICADO end-to-end (2026-06-22):** login + listado de productos funcionando
     front→SAM→Lambda→Postgres. (Dos bugs resueltos en el camino, ver §6 gotchas SAM local.)
5.5. ~~**Front de reportes (requisito #5 del reto)**~~ ✅ HECHO (2026-06-22) — el backend
   `reports` ya existía pero NO había UI. Construido:
   - `src/services/reportsService.ts` — consume `/reports/daily|top-products|critical-stock`
   - `src/app/reports/page.tsx` — Server Component, `requireRole('owner','manager')`, 3 fetch en paralelo
   - `src/app/reports/loading.tsx` + `src/components/reports/ReportsSkeleton.tsx` (skeleton separado)
   - `src/components/reports/{SummaryCards,TopProductsList,CriticalStockTable}.tsx` — UI pura
   - `navigation.ts`: módulo Reportes `available: true`. Owner ve todas las sucursales
     (columna sucursal en stock crítico); manager solo la suya.
   - Tests (8) en verde. tsc limpio. Suite completa del front: 23 tests.
5.6. ~~**Módulo de inventario (gestión completa)**~~ ✅ HECHO (2026-06-22) — extra sobre el reto.
   BACK (en la Lambda products):
   - `internal/db/queries/inventory.sql`: `ListInventoryByBranch` (LEFT JOIN, incluye productos
     sin renglón), `RestockInventory` y `SetMinStock` (ambos UPSERT con ON CONFLICT → soportan
     productos recién creados sin inventario).
   - `internal/inventoryapi/{service,handler}.go` + tests (10 en verde). Solo owner+manager.
     Manager atado a su sucursal (JWT); owner elige sucursal. Restock suma; min-stock fija umbral.
   - Montado en `cmd/products/main.go` bajo `/inventory`. Template: `InventoryRoot` + `Inventory`
     (recordar gotcha del path raíz).
   FRONT:
   - `src/services/inventoryService.ts`, `src/actions/inventory.actions.ts` (restock + setMinStock)
   - `src/hooks/useInventoryRow.ts` (lógica de fila con useTransition)
   - `src/components/inventory/{InventoryTable,InventoryRow,InventorySkeleton}.tsx`
   - `src/app/inventory/{page,loading}.tsx` — owner elige sucursal (como el POS), manager ve la suya
   - `navigation.ts`: Inventario `available: true`. Tests (4) verde. Suite front: 27 tests.
5.7. ~~**Módulos del dueño: Sucursales + Usuarios (CRUD)**~~ ✅ HECHO (2026-06-22).
   SUCURSALES (en Lambda products): branchesapi pasó de solo-lectura a CRUD completo
   (service+handler+tests, 6 en verde). GET para cualquier autenticado; POST/PATCH/DELETE
   solo owner. `?all=true` lista inactivas. PATCH con `{active}` reactiva; DELETE desactiva.
   Front: branchesService, branches.actions, BranchForm, BranchesTable, páginas list/new/edit.
   USUARIOS (NUEVO usersapi en Lambda auth): CRUD solo owner. bcrypt server-side, hash NUNCA
   expuesto. Email único, password ≥6, rol válido, sucursal coherente (owner sin sucursal;
   manager/employee la requieren). Owner no puede auto-desactivarse. Tests (9 en verde).
   Front: usersService, users.actions, UserForm (rol/sucursal condicional + password opcional
   en edición), UsersTable, páginas list/new/edit. Template: `/users` (root+proxy) en auth.
   `navigation.ts`: Sucursales + Usuarios `available: true`. Suite front: 34 tests. tsc limpio.
   ← SIGUIENTE: Deploy AWS (paso 6)
6. **Deploy AWS**: Front → S3 + CloudFront · Back → Lambdas contenedor (ECR) tras
   API Gateway vía `sam deploy --guided` · BD → RDS Postgres (+ RDS Proxy opcional).
7. **Entregables**: diagrama de arquitectura AWS · ERD (ya existe en el front,
   `docs/`) · PPT 8–10 slides.

### Patrón para CADA recurso nuevo (replicar el de `auth`)
```
1. Query SQL      → internal/db/queries/<recurso>.sql   → sqlc generate
2. Service        → internal/<recurso>api/service.go    (lógica + tests)
3. Handler        → internal/<recurso>api/handler.go    (HTTP + rutas chi)
4. Lambda main    → cmd/<recurso>/main.go                (igual que cmd/auth)
5. SAM            → agregar Function al template.yaml    (HttpApi /<recurso>/{proxy+})
```

## 6. Convenciones y gotchas

- **Identificadores en inglés, UI en español.** Valores de dominio (`paleta`, `nieve`,
  `agua_fresca`) se quedan en español a propósito.
- **Lógica de negocio en el BACK**, nunca en el front. Componentes = solo presentación.
- **Tests obligatorios** en cada pieza nueva (Go: `_test.go`; front: Vitest + Testing Library).
- **Reglas de seguridad reales** = middleware `RequireRole` en el back, no solo UI.
- **Operaciones destructivas** (rm -rf, --force, reset de BD) → el usuario las corre él,
  NO auto-ejecutar. Entregárselas como comando.
- **Dos BD durante la transición**: front Postgres en **5432** (Prisma, se va a morir),
  back Postgres en **5433** (fuente de verdad). Tienen UUIDs distintos → el nombre de
  sucursal en el dashboard del manager puede salir vacío hasta migrar la data al API.
  **Probar con `dueno@` (owner) para evitar ese desfase.**
- **Limpieza pendiente en el front** (correr cuando el login se confirme): borrar
  `src/auth.ts`, `src/auth.config.ts`, `src/app/api/auth/[...nextauth]/`,
  `src/types/next-auth.d.ts` y la dep `next-auth` de package.json.
- **`numeric`→string** en sqlc: al hacer aritmética de dinero (sales) hay que parsear.
- **Tooling instalado:** Go 1.26, SAM 1.160, Docker, AWS CLI 2, sqlc 1.31, golang-migrate 4.19.
- `env.json` (vars para `sam local`) está gitignored; contiene `host.docker.internal:5433`.

### Gotchas de SAM local (resueltos 2026-06-22 — NO repetir)
1. **`{proxy+}` NO matchea el path raíz.** API Gateway exige ≥1 segmento, así que
   `/products/{proxy+}` captura `/products/sellable` pero NO `/products` a secas → 403.
   **Fix:** por cada recurso que sirve `/` en su router chi, agregar en `template.yaml`
   un evento de path desnudo además del `{proxy+}`. Ya están: `ProductsRoot`, `BranchesRoot`,
   `SalesRoot`. (`auth` y `reports` no lo necesitan: solo sirven sub-paths.)
   Al agregar un recurso nuevo que tenga `GET /` o `POST /`, AGREGAR su `<Recurso>Root`.
2. **`env.json` debe usar la clave `Parameters`, no por-función.** Antes solo tenía
   `AuthFunction` → products/sales/reports caían al default y apuntaban a `127.0.0.1:5433`,
   que dentro del contenedor Docker es el propio contenedor → `connection refused` / 502.
   **Fix:** `{ "Parameters": { "DATABASE_URL": "...host.docker.internal:5433...", "JWT_SECRET": "..." } }`
   inyecta las vars a TODAS las Lambdas de una vez. `127.0.0.1` NUNCA funciona desde el
   contenedor; siempre `host.docker.internal` en local (y el endpoint de RDS en AWS).
3. Cambios solo en `template.yaml`/`env.json` → basta reiniciar `sam local start-api`.
   Cambios en código Go → requieren `sam build` antes de reiniciar.

## 7. Archivos clave para orientarse rápido

| Archivo | Qué es |
|---|---|
| `la-michi-pos-api/internal/authapi/{service,handler}.go` | Patrón a replicar para cada recurso |
| `la-michi-pos-api/cmd/auth/main.go` | Cómo se arma una Lambda (Lambda vs HTTP local) |
| `la-michi-pos-api/template.yaml` | Definición SAM (agregar functions aquí) |
| `la-michi-pos-api/migrations/000001_init.up.sql` | Schema completo (el ERD en SQL) |
| `la-michi-pos-api/sqlc.yaml` | Config de generación de código |
| `la-michi-pos/src/lib/{jwt,session,auth-guards}.ts` | Auth del front (cookie + jose) |
| `la-michi-pos/src/actions/auth.actions.ts` | Login/logout contra el API |
| `la-michi-pos/docs/` | ERD.md, erd.png, NOTAS.md (entregables) |
```
