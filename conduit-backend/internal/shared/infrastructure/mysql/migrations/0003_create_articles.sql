CREATE TABLE IF NOT EXISTS articles (
  id          CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  author_id   CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  slug        VARCHAR(80) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  title       VARCHAR(100) NOT NULL,
  description VARCHAR(200) NOT NULL,
  body        MEDIUMTEXT NOT NULL,
  seq         BIGINT NOT NULL AUTO_INCREMENT,
  created_at  DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  updated_at  DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  PRIMARY KEY (id),
  UNIQUE KEY articles_slug_unique (slug),
  UNIQUE KEY articles_seq_unique (seq),
  KEY articles_author (author_id),
  KEY articles_order (created_at DESC, seq DESC),
  CONSTRAINT articles_author_fk FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT articles_slug_not_feed CHECK (slug <> 'feed')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS article_tags (
  article_id CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  position   SMALLINT NOT NULL,
  tag        VARCHAR(30) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (article_id, position),
  UNIQUE KEY article_tags_article_tag_unique (article_id, tag),
  KEY article_tags_tag (tag),
  CONSTRAINT article_tags_article_fk FOREIGN KEY (article_id) REFERENCES articles (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS favorites (
  user_id    CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  article_id CHAR(36) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT (UTC_TIMESTAMP(6)),
  PRIMARY KEY (user_id, article_id),
  KEY favorites_article (article_id),
  CONSTRAINT favorites_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  CONSTRAINT favorites_article_fk FOREIGN KEY (article_id) REFERENCES articles (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
