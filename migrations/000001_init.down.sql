-- Rollback en orden inverso a las dependencias (FK).
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS inventory;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS branches;

DROP TYPE IF EXISTS sale_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS product_category;
DROP TYPE IF EXISTS user_role;
