CREATE TABLE IF NOT EXISTS users (
  id            CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  username      VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  email         VARCHAR(254) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  password_hash TEXT NOT NULL,
  bio           VARCHAR(300) NULL,
  image         TEXT NULL,
  created_at    DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  updated_at    DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  PRIMARY KEY (id),
  UNIQUE KEY users_email_unique ((LOWER(email))),
  UNIQUE KEY users_username_unique ((LOWER(username)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
