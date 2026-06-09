INSERT INTO accounts (id, name, balance) VALUES
(1, 'Alice', 100000),
(2, 'Bob', 50000)
ON CONFLICT (id) DO NOTHING;
