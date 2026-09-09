-- One versioned document: CAS keeps scheduling, publication and settings atomic.
CREATE TABLE IF NOT EXISTS companion_state (
 id INTEGER PRIMARY KEY CHECK(id=1),
 revision INTEGER NOT NULL,
 document TEXT NOT NULL,
 migration TEXT NOT NULL
);
