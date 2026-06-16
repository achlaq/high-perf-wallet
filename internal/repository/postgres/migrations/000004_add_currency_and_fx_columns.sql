-- 1. Add currency column to accounts table
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS currency VARCHAR(3) NOT NULL DEFAULT 'IDR';

-- 2. Add new columns to transfers table
ALTER TABLE transfers ADD COLUMN IF NOT EXISTS source_currency VARCHAR(3) NOT NULL DEFAULT 'IDR';
ALTER TABLE transfers ADD COLUMN IF NOT EXISTS target_currency VARCHAR(3) NOT NULL DEFAULT 'IDR';
ALTER TABLE transfers ADD COLUMN IF NOT EXISTS source_amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE transfers ADD COLUMN IF NOT EXISTS target_amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE transfers ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(15, 6) NOT NULL DEFAULT 1.0;

-- 3. Populate new columns from existing data
UPDATE transfers SET 
    source_amount = amount, 
    target_amount = amount;

-- 4. Drop the old amount column
ALTER TABLE transfers DROP COLUMN IF EXISTS amount;

-- 5. Seed some accounts with different currencies
INSERT INTO accounts (id, name, balance, currency) VALUES
(3, 'Charlie', 50000, 'USD'),
(4, 'David', 100000, 'SGD')
ON CONFLICT (id) DO NOTHING;
