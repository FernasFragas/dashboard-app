# ADR-009: Parse plan documents at upload, never from stored source at boot

| | |
| --- | --- |
| **Status** | DECIDED |
| **Date** | 2026-08-22 |
| **Author** | @FernasFragas |
| **Related** | ADR-004, docs/DATABASE.md §8, docs/PLAN-FORMAT.md |

## 1. Context and Problem Statement

The default app boots from the committed `internal/seed/seed.json`, not directly from markdown.
That keeps parser bugs from making the dashboard fail to start.

M11 adds browser plan upload. The server now must parse markdown, but only when the user asks it
to preview or apply a document.

## 2. Decision

Plan markdown uploaded through the UI is parsed only on explicit requests to
`POST /api/plan/preview` and `POST /api/plan/apply`.

Stored uploaded sources are retained for audit/export, but they are not parsed during normal
startup. A corrupt stored source can make preview/apply fail; it cannot prevent boot.

## 3. Consequences

- Normal boot still depends on migrations plus the seed document, not a markdown parser.
- Bad markdown is reported in the browser with line numbers and changes no rows.
- Applying a replacement plan takes a SQLite snapshot before mutating plan-owned rows.
- The existing explicit `-plan` CLI path remains a development/import path; it is not used by
  browser-loaded stored sources.
