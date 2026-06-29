# ⚠️ DEPRECATED — reference only, not executed

As of 2026-06-25 these JSON files are **superseded** and are **not loaded by any
test or gate**. They are kept only as scenario reference for authoring real cases.

Why they were retired:
- They were **orphaned** — no Go code ever loaded them.
- Every case carries a **pre-baked `actual_output` + `evaluation_results`**, i.e.
  it was "scored" ahead of time rather than at evaluation. A golden case must
  carry only `input` + `expected`; the verdict is produced at run time.
- Their `input` keys do **not** map to the judges' input contracts, so they
  cannot be scored by the real deterministic judges.
- The `INDEX.json` totals (claims 300; headers sum to 220; files actually
  contain 93) never matched reality.

## Use this instead

The single source of truth is **`pkg/eval/golden/`**:

- `data/*.json` — real, correctly-keyed cases (settlement, compliance,
  orchestration, lineage), loaded by `golden.LoadGoldenDir`.
- `golden.EvaluateCase` — scores a case with the deterministic judge for its
  domain (typed dispatch; **not** the dead `judges.Judge` interface).
- `golden.TestGoldenGate` — the honest, fail-capable regression gate.

To add coverage, author a case in `pkg/eval/golden/data/<domain>.json` using the
keys the corresponding `judges.<Domain>JudgeInput` expects, and assert the
verdict the deterministic judge will reach. See `EVAL_EXPANSION_PLAN.md` Phase 2
for the prioritized backlog (compliance/KYC/AML first).

Migration of any still-useful scenarios from these files into the new format is
tracked as Phase 2 work.
