CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, enabled INTEGER NOT NULL,
 lastSeen TEXT NOT NULL, status TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS project_files (
 project TEXT NOT NULL, path TEXT NOT NULL, content TEXT NOT NULL,
 revision TEXT NOT NULL, writer TEXT NOT NULL, updatedAt TEXT NOT NULL,
 bytes INTEGER NOT NULL, macRevision TEXT NOT NULL DEFAULT '', macIssue TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(project,path)
);
CREATE TABLE IF NOT EXISTS project_revisions (
 project TEXT NOT NULL, revision TEXT NOT NULL, path TEXT NOT NULL,
 content TEXT NOT NULL, baseRevision TEXT NOT NULL, writer TEXT NOT NULL, updatedAt TEXT NOT NULL,
 PRIMARY KEY(project,revision)
);
CREATE INDEX IF NOT EXISTS project_history_path ON project_revisions(project,path);
CREATE TRIGGER IF NOT EXISTS project_revision_applied AFTER INSERT ON project_revisions BEGIN
 INSERT INTO project_files(project,path,content,revision,writer,updatedAt,bytes)
 VALUES(NEW.project,NEW.path,NEW.content,NEW.revision,NEW.writer,NEW.updatedAt,length(CAST(NEW.content AS BLOB)))
 ON CONFLICT(project,path) DO UPDATE SET content=excluded.content,revision=excluded.revision,
 writer=excluded.writer,updatedAt=excluded.updatedAt,bytes=excluded.bytes,macIssue='';
END;
