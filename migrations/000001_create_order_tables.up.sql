-- =========================================
-- Set default timezone
-- =========================================
SET TIME ZONE 'Asia/Almaty';

-- =========================================
-- Orders table
-- =========================================
CREATE TABLE IF NOT EXISTS orders (
    id               SERIAL PRIMARY KEY,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    number           TEXT UNIQUE NOT NULL,
    customer_name    TEXT NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('dine_in', 'takeout', 'delivery')),
    table_number     INTEGER,
    delivery_address TEXT,
    total_amount     DECIMAL(10,2) NOT NULL,
    priority         INTEGER DEFAULT 1,
    status           TEXT DEFAULT '',
    processed_by     TEXT,
    completed_at     TIMESTAMPTZ
    );

-- =========================================
-- Order items table
-- =========================================
CREATE TABLE IF NOT EXISTS order_items (
    id          SERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    order_id    INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    quantity    INTEGER NOT NULL CHECK (quantity > 0),
    price       DECIMAL(8,2) NOT NULL CHECK (price >= 0)
    );

-- =========================================
-- Order status log table
-- =========================================
CREATE TABLE IF NOT EXISTS order_status_log (
    id          SERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    order_id    INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,
    changed_by  TEXT,
    changed_at  TIMESTAMPTZ DEFAULT current_timestamp,
    notes       TEXT
    );

-- =========================================
-- Useful indexes
-- =========================================
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_status_log_order_id ON order_status_log(order_id);

-- =========================================
-- Trigger to automatically update "updated_at"
-- =========================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'set_orders_updated_at'
  ) THEN
CREATE TRIGGER set_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
END IF;
END;
$$;

-- =========================================
-- Daily order number generation
-- Format: ORD_YYYYMMDD_NNN
-- =========================================

-- Store counters per day
CREATE TABLE IF NOT EXISTS order_counters (
    order_date DATE PRIMARY KEY,
    counter    INT NOT NULL
);

-- Function to generate order number
CREATE OR REPLACE FUNCTION generate_order_number()
RETURNS TEXT AS $$
DECLARE
today DATE := (now() AT TIME ZONE 'UTC')::date;
  seq   INT;
BEGIN
  LOOP
    -- Try to increment existing counter
UPDATE order_counters
SET counter = counter + 1
WHERE order_date = today
    RETURNING counter INTO seq;

-- If updated, exit loop
IF FOUND THEN
      EXIT;
END IF;

    -- Otherwise insert new counter starting from 1
BEGIN
INSERT INTO order_counters(order_date, counter) VALUES (today, 1)
    RETURNING counter INTO seq;
EXIT;
EXCEPTION WHEN unique_violation THEN
      -- If another transaction inserted same row, retry
END;
END LOOP;

RETURN format('ORD_%s_%03s', to_char(today, 'YYYYMMDD'), seq);
END;
$$ LANGUAGE plpgsql;

-- Trigger to assign order number on insert
CREATE OR REPLACE FUNCTION set_order_number()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.number IS NULL THEN
    NEW.number := generate_order_number();
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'set_order_number_trigger'
  ) THEN
CREATE TRIGGER set_order_number_trigger
    BEFORE INSERT ON orders
    FOR EACH ROW
    EXECUTE FUNCTION set_order_number();
END IF;
END;
$$;

-- =========================================
-- Example inserts for testing
-- =========================================

-- Insert a new order (number will be generated automatically)
INSERT INTO orders (customer_name, type, total_amount)
VALUES ('John Doe', 'takeout', 25.50)
    RETURNING id, number, created_at;

-- Insert an order item linked to the order
-- (replace :order_id with the actual id from previous insert)
-- INSERT INTO order_items (order_id, name, quantity, price)
-- VALUES (:order_id, 'Pepperoni Pizza', 2, 12.75);
