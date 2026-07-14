-- 001_init.sql — schema inicial do FeedClaw
-- Fonte de verdade única do read state e do conteúdo cacheado.

CREATE TABLE feeds (
  id            INTEGER PRIMARY KEY,
  url           TEXT NOT NULL UNIQUE,        -- xmlUrl
  site_url      TEXT,
  title         TEXT NOT NULL,
  category      TEXT NOT NULL DEFAULT '',    -- pasta do OPML/Feedly
  etag          TEXT,
  last_modified TEXT,
  last_fetch_at DATETIME,
  last_status   INTEGER,                     -- último HTTP status
  error_count   INTEGER NOT NULL DEFAULT 0,  -- health check
  disabled      INTEGER NOT NULL DEFAULT 0,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE articles (
  id           INTEGER PRIMARY KEY,
  feed_id      INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid         TEXT NOT NULL,               -- guid do item ou fallback: hash(url+title)
  url          TEXT NOT NULL,
  title        TEXT NOT NULL,
  summary      TEXT,                        -- description/summary do feed
  content      TEXT,                        -- content:encoded quando vier no feed
  full_content TEXT,                        -- readability cache (lazy)
  author       TEXT,
  published_at DATETIME,
  fetched_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  read_at      DATETIME,                    -- NULL = não lido
  starred      INTEGER NOT NULL DEFAULT 0,  -- "ler depois" explícito
  UNIQUE (feed_id, guid)
);

CREATE INDEX idx_articles_unread ON articles (read_at) WHERE read_at IS NULL;
CREATE INDEX idx_articles_published ON articles (published_at DESC);
CREATE INDEX idx_articles_feed ON articles (feed_id);

-- FTS5 externa (content='articles') indexando title, summary, full_content.
CREATE VIRTUAL TABLE articles_fts USING fts5(
  title, summary, full_content,
  content='articles', content_rowid='id'
);

-- Triggers de sincronização da FTS externa.
CREATE TRIGGER articles_ai AFTER INSERT ON articles BEGIN
  INSERT INTO articles_fts (rowid, title, summary, full_content)
  VALUES (new.id, new.title, new.summary, new.full_content);
END;

CREATE TRIGGER articles_ad AFTER DELETE ON articles BEGIN
  INSERT INTO articles_fts (articles_fts, rowid, title, summary, full_content)
  VALUES ('delete', old.id, old.title, old.summary, old.full_content);
END;

CREATE TRIGGER articles_au AFTER UPDATE ON articles BEGIN
  INSERT INTO articles_fts (articles_fts, rowid, title, summary, full_content)
  VALUES ('delete', old.id, old.title, old.summary, old.full_content);
  INSERT INTO articles_fts (rowid, title, summary, full_content)
  VALUES (new.id, new.title, new.summary, new.full_content);
END;

CREATE TABLE digests (
  id           INTEGER PRIMARY KEY,
  date         TEXT NOT NULL UNIQUE,        -- YYYY-MM-DD
  generated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  model_note   TEXT                         -- opcional: qual agente/modelo gerou
);

CREATE TABLE digest_themes (
  id        INTEGER PRIMARY KEY,
  digest_id INTEGER NOT NULL REFERENCES digests(id) ON DELETE CASCADE,
  position  INTEGER NOT NULL,               -- ordem de exibição
  name      TEXT NOT NULL,
  summary   TEXT NOT NULL
);

CREATE TABLE digest_theme_articles (
  theme_id   INTEGER NOT NULL REFERENCES digest_themes(id) ON DELETE CASCADE,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  PRIMARY KEY (theme_id, article_id)
);
