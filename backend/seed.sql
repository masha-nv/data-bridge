CREATE TABLE IF NOT EXISTS beneficiaries (
  bene_id TEXT PRIMARY KEY,
  bene_name TEXT
);

INSERT INTO beneficiaries (bene_id, bene_name) VALUES
  ('123', 'Alice'),
  ('456', 'Bob');


CREATE TABLE IF NOT EXISTS users (
    user_id TEXT PRIMARY KEY,
    user_name  TEXT
);

INSERT INTO users (user_id, user_name) VALUES 
('1', 'Masha'),
('2', 'Katya');