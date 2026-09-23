-- SPDX-License-Identifier: AGPL-3.0-only
-- Copyright (C) 2026 FireBall1725

CREATE TABLE scans (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at  TEXT    NOT NULL,
    finished_at TEXT    NOT NULL,
    state       TEXT    NOT NULL,
    error       TEXT    NOT NULL DEFAULT '',
    commit_sha  TEXT    NOT NULL DEFAULT '',
    apps        INTEGER NOT NULL DEFAULT 0,
    checked     INTEGER NOT NULL DEFAULT 0,
    updates     INTEGER NOT NULL DEFAULT 0,
    errors      INTEGER NOT NULL DEFAULT 0,
    -- The full result as JSON, kept only on the newest successful scan so a restart has something to show.
    result      TEXT
);

CREATE TABLE skips (
    app_dir    TEXT NOT NULL,
    field      TEXT NOT NULL,
    version    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (app_dir, field, version)
);

CREATE TABLE pull_requests (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL,
    title      TEXT    NOT NULL,
    branch     TEXT    NOT NULL,
    state      TEXT    NOT NULL,
    auto_merge INTEGER NOT NULL DEFAULT 0,
    number     INTEGER NOT NULL DEFAULT 0,
    url        TEXT    NOT NULL DEFAULT '',
    error      TEXT    NOT NULL DEFAULT '',
    -- The bumps in this PR as JSON: app dir, field, from and to.
    items      TEXT    NOT NULL
);
