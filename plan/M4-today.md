# M4 · Today screen + quick log

**Depends on:** M3. **Unblocks:** M5, M6 (both reuse the shell and the query layer).
**Parent:** [../dashboard-plan-v3.md](../dashboard-plan-v3.md) · **Rationale:** `dashboard-plan-v2.md` §1 "Screens", §2

First frontend milestone: it establishes the app shell, the query layer, and the mutation
conventions that M5 and M6 inherit. Get these right here and the later screens are assembly.

## Shell (built once, used by all four screens)

- Routing: `wouter`. Routes `/` (Today) · `/goals` · `/log` · `/review`.
- Server state: TanStack Query. One `QueryClient`; `src/api/client.ts` wraps fetch, attaches
  `X-Token` when configured, and turns the `{"error": …}` envelope into a thrown typed error
  carrying the status.
- Navigation: bottom tab bar on phone, left rail from `md:` up. Dark theme only, palette as
  `@theme` tokens in `index.css`.
- Tap targets ≥44px. The + bar sits within thumb reach — bottom third of the screen.
- Desktop shortcut `q` opens the quick-log sheet.

## Today screen

1. **Week banner** — plan week + focus from `GET /api/dashboard` ("W5 · Chaos: fail-open /
   fail-static"). Renders nothing when the API returns `state: not_started | plan_complete`.
2. **Rhythm slot** — today's line from the operating-system table ("Thu — SAA prep + 30 min
   community").
3. **Task checklist** — this week's tasks. Each row: title + project tag; tap the row body to
   expand `steps[]` and `done_means`; tap the checkbox to toggle. Checking is optimistic with
   instant strikethrough.
4. **Project chips** — `synapse · gateway · dash · oss · learn · write · career`.
   **Multi-select**, state persisted in the URL query string (`/?project=synapse,gateway`), so
   a reload keeps the filter and the phone home-screen icon can deep-link to a project.
5. **Counters + streak** — "This week: 3 applications · 5 modules · 1 number" and the streak
   chip, dimmed until `counts_today` is true.

## The + bar — 8 category buttons, always visible

A permanent 4×2 grid on Today: Application 📮 · Module 📚 · Portfolio 🔧 · Post ✍️ · OSS 🔀 ·
Number 📊 · Network 🤝 · Exam 🎓.

Tap a category → bottom sheet opens **with that category already selected**. Fields:

| Field | Required | Notes |
|---|---|---|
| Title | yes | autofocused, keyboard up on open |
| Note / URL | no | one field; a value starting `http` renders as a link in the feed |
| Link to goal | no | goal picker — this is what feeds the "proof of motion" chip on Kanban cards |
| Occurred at | no | defaults to now (Lisbon); tappable to backdate |

Save → `POST /api/logs` → sheet closes → counters and feed invalidate.

**Tap budget:** category → title → Save = 3 taps plus typing. The optional fields are the only
thing between that and the <5-tap target — they must never steal focus or block Save.

## Mutation conventions (inherited by M5/M6)

- Optimistic: task toggle (here) and Kanban move (M5). Everything else waits for the 200.
- Every `PATCH` sends `If-Match` with the resource's `version` from the last read.
- **On 412:** invalidate and re-render with server truth, **and** show a toast —
  "Updated elsewhere · refreshed". The checkbox silently flipping back reads as a bug; the
  toast says it wasn't.
- On any other error: roll back the optimistic update, toast the message from the error envelope.

## Tests

Vitest + Testing Library on the pieces where the logic actually lives:

1. Optimistic toggle: renders checked immediately, rolls back on a 500.
2. 412 path: toggle → 412 → checkbox reflects server state → toast rendered.
3. Project chips: multi-select writes to the URL; mounting with `?project=…` applies the filter.
4. Quick-log sheet: Save disabled on a blank title; a successful save closes the sheet.

## Done when

1. On the phone over Tailscale, logging a job application takes **fewer than 5 taps** and
   survives a reload.
2. Checking a W1 task persists across a server restart.
3. `?project=synapse` deep-links to a filtered Today.
