package db

const schemaVersion = 7

var migrations = [][]string{
	// v1: Initial schema - each statement separate for SQLite compatibility
	{
		`CREATE TABLE IF NOT EXISTS planning_state (
			issue_key TEXT PRIMARY KEY,
			conversation_json TEXT NOT NULL DEFAULT '[]',
			participants_json TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'active',
			original_description TEXT NOT NULL DEFAULT '',
			figma_urls_json TEXT DEFAULT '[]',
			image_refs_json TEXT DEFAULT '[]',
			last_human_response_at TEXT,
			last_system_comment_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pr_feedback_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			issue_key TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			comment_id TEXT NOT NULL UNIQUE,
			comment_type TEXT NOT NULL,
			action_taken TEXT NOT NULL,
			commit_sha TEXT,
			processed_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_feedback_pr ON pr_feedback_state(pr_number)`,
		`CREATE INDEX IF NOT EXISTS idx_pr_feedback_issue ON pr_feedback_state(issue_key)`,
		`CREATE TABLE IF NOT EXISTS feedback_cutoffs (
			issue_key TEXT PRIMARY KEY,
			cutoff_utc TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS processed_shas (
			sha TEXT PRIMARY KEY,
			processed_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attempt_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			issue_key TEXT NOT NULL,
			attempted_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_attempt_issue ON attempt_records(issue_key)`,
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER NOT NULL
		)`,
		`INSERT INTO schema_version (version) VALUES (1)`,
	},
	// v2: Description-centric planning flow
	{
		`ALTER TABLE planning_state ADD COLUMN bot_comment_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE planning_state ADD COLUMN last_seen_description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE planning_state ADD COLUMN questions_json TEXT NOT NULL DEFAULT '[]'`,
	},
	// v3: Two-phase planning (product refinement → technical refinement)
	{
		`ALTER TABLE planning_state ADD COLUMN planning_phase TEXT NOT NULL DEFAULT 'product'`,
	},
	// v4: Persist product refinement summary across phase transitions
	{
		`ALTER TABLE planning_state ADD COLUMN product_summary TEXT NOT NULL DEFAULT ''`,
	},
	// v5: Slack thread tracking
	{
		`CREATE TABLE IF NOT EXISTS slack_threads (
			issue_key TEXT PRIMARY KEY,
			thread_ts TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	},
	// v6: Plugin framework — add plugin_id columns and label_requests table
	{
		`ALTER TABLE planning_state ADD COLUMN plugin_id TEXT NOT NULL DEFAULT 'developer'`,
		`ALTER TABLE planning_state ADD COLUMN repo_names TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE attempt_records ADD COLUMN plugin_id TEXT NOT NULL DEFAULT 'developer'`,
		`ALTER TABLE pr_feedback_state ADD COLUMN plugin_id TEXT NOT NULL DEFAULT 'developer'`,
		`ALTER TABLE feedback_cutoffs ADD COLUMN plugin_id TEXT NOT NULL DEFAULT 'developer'`,
		`CREATE TABLE IF NOT EXISTS label_requests (
			issue_key TEXT PRIMARY KEY,
			board_id TEXT NOT NULL,
			comment_id TEXT,
			requested_at TEXT NOT NULL
		)`,
	},
	// v7: Security engineer plugin — scan and finding tables
	{
		`CREATE TABLE IF NOT EXISTS security_scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_name TEXT NOT NULL,
			scan_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			commit_hash TEXT,
			findings_count INTEGER DEFAULT 0,
			summary TEXT,
			started_at TEXT,
			completed_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS security_findings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_name TEXT NOT NULL,
			scan_id INTEGER NOT NULL REFERENCES security_scans(id),
			agent TEXT NOT NULL,
			finding_id TEXT,
			title TEXT NOT NULL,
			description TEXT,
			severity TEXT NOT NULL,
			confidence TEXT,
			priority TEXT,
			category TEXT,
			cwe_id TEXT,
			owasp_category TEXT,
			file_path TEXT,
			line_start INTEGER,
			line_end INTEGER,
			snippet TEXT,
			evidence TEXT,
			source TEXT,
			source_tool TEXT,
			remediation TEXT,
			remediation_effort TEXT,
			code_suggestion TEXT,
			false_positive_risk TEXT,
			status TEXT NOT NULL DEFAULT 'open',
			fingerprint TEXT NOT NULL,
			first_seen_scan_id INTEGER,
			last_seen_scan_id INTEGER,
			jira_issue_key TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(repo_name, fingerprint)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_security_findings_repo ON security_findings(repo_name)`,
		`CREATE INDEX IF NOT EXISTS idx_security_findings_status ON security_findings(status)`,
		`CREATE INDEX IF NOT EXISTS idx_security_findings_severity ON security_findings(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_security_scans_repo ON security_scans(repo_name)`,
	},
}
