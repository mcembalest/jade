-- Every accepted edit is immutable. The trigger advances the current document
-- in the same transaction, so a failed/interrupted request can be retried safely.
CREATE TABLE IF NOT EXISTS revisions (
  revision TEXT PRIMARY KEY,
  path TEXT NOT NULL,
  content TEXT NOT NULL,
  baseRevision TEXT NOT NULL,
  writer TEXT NOT NULL,
  updatedAt TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS files (
  path TEXT PRIMARY KEY,
  content TEXT NOT NULL,
  revision TEXT NOT NULL,
  writer TEXT NOT NULL,
  updatedAt TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS acknowledgements (
  path TEXT NOT NULL,
  deviceId TEXT NOT NULL,
  revision TEXT NOT NULL,
  PRIMARY KEY (path, deviceId)
);
CREATE TRIGGER IF NOT EXISTS revision_applied AFTER INSERT ON revisions BEGIN
  INSERT INTO files(path, content, revision, writer, updatedAt)
  VALUES(NEW.path, NEW.content, NEW.revision, NEW.writer, NEW.updatedAt)
  ON CONFLICT(path) DO UPDATE SET content=excluded.content, revision=excluded.revision,
    writer=excluded.writer, updatedAt=excluded.updatedAt;
END;

CREATE TABLE IF NOT EXISTS remote_requests (
 id TEXT PRIMARY KEY,
 payload TEXT NOT NULL,
 result TEXT,
 created INTEGER NOT NULL
);
