## What and why

<!-- Link the task brief or milestone. One reviewable concern per pull request. -->

## Risk

<!-- docs/agents/PLAYBOOK.md §4 -->

- [ ] Low: docs, copy or tests only
- [ ] Medium: handlers, store, UI
- [ ] High: migrations, seed or plan, pairing or auth, backup, build or CI, release

## Evidence

<!-- Commands and results. For UI: what you checked on make dev-instance, and at which widths. -->

- [ ] `make check-ci` passes
- [ ] Behaviour verified on an isolated instance (`make dev-instance`)
- [ ] Self-review and independent-review findings resolved (list anything deliberately left open)

## Docs changed with the code

- [ ] `docs/API.md` for endpoints, `docs/DATABASE.md` for schema, `docs/PROGRESS.md` for status,
      `docs/TECH_DEBT.md` for new debt — or not applicable

## Human validation

- [ ] **Correctness:** matches "done means"; Lisbon day boundaries, `412` conflicts and an empty plan considered
- [ ] **Security:** nothing binds `0.0.0.0`; no tokens or pairing codes logged; input validated
- [ ] **Performance:** no per-row queries in list handlers; no unbounded responses
- [ ] **Maintainability:** layering holds; no duplicate helpers; tests are readable
- [ ] **Requirements:** nothing out of scope; migrations are new files only
