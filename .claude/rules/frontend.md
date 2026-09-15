---
paths:
  - "web/**"
---

# Frontend rules (`web/`)

## Layout

```
src/App.tsx             QueryClient, app shell, routes: / /goals /log /review /skills /plan
src/api/client.ts       every HTTP call + response types (snake_case, mirrors docs/API.md)
src/pages/*Page.tsx     one screen each, with a co-located *.test.tsx
src/pages/goalBoard.ts  pure Kanban move logic (resolveDropTarget → planMove → applyMove)
src/components/         FieldGuide, GameSkillBar, LevelUpMoment
src/copy/*.ts           user-facing strings, one file per screen/feature
src/lib/date.ts         Europe/Lisbon date helpers (lisbonDateKey, …)
src/lib/motion.ts       usePrefersReducedMotion
src/lib/queryKeys.ts    query keys shared by more than one page
src/index.css           Tailwind import + @theme tokens (dark palette)
public/                 PWA manifest + icons
```

pnpm only. The lockfile is committed and CI installs with `--frozen-lockfile`.

## Conventions

- **All HTTP goes through `src/api/client.ts`** (`apiFetch`); ESLint rejects `fetch` anywhere
  else. It attaches `X-Token` from
  `localStorage["dashboard.token"]` or `VITE_DASHBOARD_TOKEN` and throws
  `ApiError { status, current }`. Add a typed function there; never `fetch` from a component.
- **Keep types in sync with the backend.** A response-shape change needs the TypeScript change
  in the same piece of work, and `docs/API.md` updated.
- **Query keys** used by two or more pages live in `src/lib/queryKeys.ts`. Two keys for the same
  endpoint once split the cache between Today and the Kanban.
- **Mutations are optimistic.** `onMutate` snapshots and `setQueryData`. `onError`: on
  `ApiError` with status 412, adopt `error.current` and invalidate; otherwise restore the snapshot
  and surface the error. After writes that can award XP, invalidate `gameProfileQueryKey`,
  `gameSkillsQueryKey` and `gameEventsQueryKey`. `TodayPage.tsx` is the reference pattern.
- **Never compute Kanban `sort_order` on the client.** Apply the server's `reordered` array.
- **Strings live in `src/copy/`**, not inline in JSX.
- **Dates:** key and group by Lisbon date using `src/lib/date.ts`. Never
  `toISOString().slice(0, 10)`: that is the UTC day.
- **Plan vocabulary** (projects, categories, skills, weeks) comes from the API. Never hardcode ids.
- **Creating a task** targets the dashboard's `task_week.code`, not `week.code`, which is null
  outside the plan window.
- **Styling:** Tailwind utilities; tokens in `@theme` in `index.css`; no `tailwind.config.js`.
  Dark theme only.
- **Mobile first:** tap targets ≥44px (`min-h-11`); bottom tab bar on phones, left rail from
  `md:`. Drag-and-drop is desktop only; phones move cards with the tap-to-move sheet.
- **Motion:** honour `usePrefersReducedMotion()`. No sound.
- **No service worker.** Offline should fail plainly, not show stale data.
- **Game copy never punishes:** no decay, no guilt messaging, no red inactivity states (M9).

## Formatting and lint

Prettier: `printWidth 100`, double quotes, semicolons, trailing commas. ESLint flat config with
typescript-eslint, react-hooks and react-refresh. `pnpm lint`, `pnpm typecheck`,
`pnpm format:check`.

## Tests

Vitest + jsdom + Testing Library + user-event; `src/test/setup.ts` loads jest-dom matchers.

Page tests mock the client module and keep everything else real:

```ts
vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return { ...actual, getDashboard: vi.fn(), updateTaskStatus: vi.fn() };
});
```

Render inside a fresh `QueryClientProvider` and type fixtures as `api.*` types so API drift fails
typecheck. Pure logic (`goalBoard.ts`, `date.ts`) gets plain unit tests.

Unit tests do not prove dnd-kit wiring: cross-column drag was once broken with every test green.
Drag changes need a manual pass (`plan/M5-followups.md` F3).

To see a change running, use `make dev-instance` (skill `run-dashboard`), not `make dev`: it points
the Vite proxy at an isolated API through `DASHBOARD_API_URL`.
