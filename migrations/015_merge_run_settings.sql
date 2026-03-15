-- 015_merge_run_settings.sql
-- SC-003 (phase 1): create consolidated run_setting table and backfill flags/rules.
--
-- NOTE: Non-destructive pass. Legacy rule/flag tables are retained until all
-- read/write paths are migrated.

CREATE TABLE IF NOT EXISTS run_setting (
    id     INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES run(id),
    type   TEXT NOT NULL CHECK(type IN ('flag','rule')),
    key    TEXT NOT NULL,
    value  TEXT NOT NULL DEFAULT 'true',
    UNIQUE(run_id, type, key)
);

INSERT OR REPLACE INTO run_setting (run_id, type, key, value)
SELECT rf.run_id, 'flag', rf.key, rf.value
FROM run_flag rf;

INSERT OR REPLACE INTO run_setting (run_id, type, key, value)
SELECT rr.run_id, 'rule', rd.key, rr.params_json
FROM run_rule rr
JOIN rule_def rd ON rd.id = rr.rule_def_id
WHERE rr.enabled = 1;

CREATE INDEX IF NOT EXISTS idx_run_setting_lookup ON run_setting(run_id, type, key);
