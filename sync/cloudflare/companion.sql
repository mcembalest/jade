-- One versioned document: CAS keeps scheduling, publication and settings atomic.
CREATE TABLE IF NOT EXISTS companion_state (
 id INTEGER PRIMARY KEY CHECK(id=1),
 revision INTEGER NOT NULL,
 document TEXT NOT NULL,
 migration TEXT NOT NULL
);

-- Permanent journal. The bounded client feed can be trimmed without losing history.
CREATE TABLE IF NOT EXISTS companion_archive (
 seq INTEGER PRIMARY KEY AUTOINCREMENT,
 id TEXT NOT NULL UNIQUE,
 kind TEXT NOT NULL,
 foundAt INTEGER NOT NULL,
 document TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS companion_archive_kind ON companion_archive(kind,seq);
CREATE TRIGGER IF NOT EXISTS companion_archive_insert AFTER INSERT ON companion_state BEGIN
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'message-' || COALESCE(json_extract(value,'$.id'),hex(value)),
 CASE WHEN json_extract(value,'$.proactive')=1 THEN 'daily' ELSE 'chat' END,
 COALESCE(json_extract(value,'$.foundAt'),0),value FROM json_each(NEW.document,'$.messages');
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'finding-' || COALESCE(json_extract(value,'$.id'),hex(value)), 'finding',
 COALESCE(json_extract(value,'$.foundAt'),0),value FROM json_each(NEW.document,'$.pending');
END;
CREATE TRIGGER IF NOT EXISTS companion_archive_update AFTER UPDATE ON companion_state BEGIN
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'message-' || json_extract(NEW.document,'$.queuedDaily.id'),'daily',
 json_extract(NEW.document,'$.queuedDaily.foundAt'),json_extract(NEW.document,'$.queuedDaily')
 WHERE json_extract(NEW.document,'$.queuedDaily.id') IS NOT NULL;
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'message-' || COALESCE(json_extract(value,'$.id'),hex(value)),
 CASE WHEN json_extract(value,'$.proactive')=1 THEN 'daily' ELSE 'chat' END,
 COALESCE(json_extract(value,'$.foundAt'),0),value FROM json_each(OLD.document,'$.messages');
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'finding-' || COALESCE(json_extract(value,'$.id'),hex(value)), 'finding',
 COALESCE(json_extract(value,'$.foundAt'),0),value FROM json_each(OLD.document,'$.pending');
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'message-' || COALESCE(json_extract(value,'$.id'),hex(value)),
 CASE WHEN json_extract(value,'$.proactive')=1 THEN 'daily' ELSE 'chat' END,
 COALESCE(json_extract(value,'$.foundAt'),0),value FROM json_each(NEW.document,'$.messages');
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'finding-' || COALESCE(json_extract(value,'$.id'),hex(value)), 'finding',
 COALESCE(json_extract(value,'$.foundAt'),0),value FROM json_each(NEW.document,'$.pending');
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'notebook-' || json_extract(NEW.document,'$.notebook.revision'),'notebook',
 COALESCE(json_extract(NEW.document,'$.notebook.updatedAt'),0),json_extract(NEW.document,'$.notebook')
 WHERE json_extract(NEW.document,'$.notebook.revision') IS NOT NULL;
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'memory-proposal-' || json_extract(NEW.document,'$.memoryProposal.id'),'notebook',
 json_extract(NEW.document,'$.memoryProposal.foundAt'),json_extract(NEW.document,'$.memoryProposal')
 WHERE json_extract(NEW.document,'$.memoryProposal.id') IS NOT NULL;
 INSERT OR IGNORE INTO companion_archive(id,kind,foundAt,document)
 SELECT 'run-' || json_extract(NEW.document,'$.run.id') || '-' || json_extract(NEW.document,'$.run.status'),'run',
 COALESCE(json_extract(NEW.document,'$.run.at'),0),json_object('text',COALESCE(json_extract(NEW.document,'$.researchError'),''),'run',json_extract(NEW.document,'$.run'))
 WHERE json_extract(NEW.document,'$.run.id') IS NOT NULL;
END;
