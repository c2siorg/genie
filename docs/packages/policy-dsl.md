# pkg/policy/dsl — board-rule expression DSL

> **Where:** `pkg/policy/dsl/` · **Lines of code:** ~330 · **Tests:** 11
> **FREE-AI alignment:** Rec 6 (Adaptive Policies), Rec 14 (Board-Approved Policy)

---

## Overview

A tiny expression DSL for board-approved governance rules. The use case:
**FREE-AI Rec 6 (Adaptive Policies)** wants the risk team to be able to
add a new rule without engineering involvement. Today every rule in
`pkg/governance` is a Go struct — the team has to ship code. With this
DSL the risk team drops a YAML file in `config/policies/` and the
system loads it at boot.

The DSL deliberately covers a small surface — comparison, boolean
composition, string `contains` / `startsWith`, dotted metadata access.
If a rule needs more (transactions, joins, regex), write a Go-side
`governance.Policy`. The goal is "let the risk team own 80 % of rules"
not "let the risk team write Turing-complete code."

Inspired by Google ADK `policy-as-code`. CEL (`github.com/google/cel-go`)
was considered but rejected to keep the dependency graph small. If
cel-go is added later, this package's `Evaluator` can be swapped for it
without changing callers.

---

## Surface

```go
type Rule struct {
    ID       string // unique
    When     string // boolean expression in the DSL
    Decision string // "allow" | "deny"
    Reason   string // human-readable rationale
}

func Compile(rules []Rule) ([]CompiledRule, error)
func AsPolicies(rules []CompiledRule) []governance.Policy
```

`CompiledRule.AsPolicy()` adapts a single rule to the existing
`governance.Policy` interface. The host loader does:

```go
raw, _ := os.ReadFile("config/policies/rules.yaml")
var rules []dsl.Rule
yaml.Unmarshal(raw, &rules)
compiled, _ := dsl.Compile(rules)
composite := governance.NewComposite(append(coreStack, dsl.AsPolicies(compiled)...)...)
```

---

## Grammar

```
expr     = orExpr
orExpr   = andExpr ("OR" andExpr)*
andExpr  = unary ("AND" unary)*
unary    = "NOT" unary | primary
primary  = "(" expr ")" | cmp
cmp      = term op term
term     = stringLit | numLit | identifier
op       = "==" | "!=" | "<" | ">" | "<=" | ">=" | "contains" | "startsWith"
```

Tokeniser: whitespace + parens are delimiters; double or single quotes
preserve string literals; `(` and `)` are their own tokens.

---

## Field references

The right-hand `identifier` resolves against the `protocol.Message`:

| Identifier | Resolves to |
|---|---|
| `classification` | `msg.Metadata["classification"]` (the message's sensitivity tier) |
| `type` | `msg.Type` |
| `from` | `msg.From` |
| `to` | `msg.To` |
| `role` | string of `msg.Role` |
| `content` | `msg.Content` |
| `metadata.<key>` | `msg.Metadata[key]` (string-coerced) |

Anything else returns an error from `expr.eval()`.

---

## Literals

- Single- or double-quoted strings: `"pii"`, `'in'`.
- Numbers: `100000`, `4.5`. Numeric compare uses `strconv.ParseFloat`.
- Booleans embedded in metadata are stringified as `"true"` / `"false"`.

---

## Example — three classic rules

```yaml
# config/policies/board.yaml
- id: deny_offshore_pii
  when: classification == "pii" AND metadata.region != "in"
  decision: deny
  reason: "PII bound for non-home region"

- id: block_sql_injection_in_content
  when: content contains "DROP TABLE"
  decision: deny
  reason: "Likely SQL injection in user content"

- id: external_partner_trace_required
  when: from startsWith "ext-" AND NOT metadata.trace_id == "present"
  decision: deny
  reason: "External-partner traffic must carry trace_id"
```

After load, every message that hits the bus is evaluated against these
on top of the Go-side composite (RBAC, length, residency, etc.). A deny
records an `IncidentPayload` and increments the denial counter.

---

## What it does NOT do

- **No SQL-like joins.** One message at a time; no cross-message rules.
- **No regex.** Use a Go-side `governance.PIIBlockPolicy` or write a CEL adapter.
- **No date / time arithmetic.** Use a Go-side policy if you need "within last 5 minutes."
- **No nested struct access beyond `metadata.<key>`.** No `metadata.user.roles`.
- **No write actions.** Decision is allow/deny only — never "mutate the message."

---

## Anti-patterns

1. **Putting Turing-complete logic in YAML.** The DSL is intentionally small. If you need joins, dates, regex, write a Go-side `governance.Policy`.
2. **Hot-reloading without an audit log.** Every reload should record the YAML file's hash + the timestamp.
3. **Treating DSL deny as soft.** Deny is deny — composite stops at first deny. Don't add "shadow" DSL rules and hope they get logged but not enforced.

---

## FREE-AI alignment

- **Rec 6 (Adaptive Policies)** — the DSL *is* this recommendation, in code form.
- **Rec 14 (Board-Approved Policy)** — the YAML file is what the board signs off on; the loader hashes it at boot.
- **Rec 23 (Audit Framework)** — every DSL deny records the `Rule.ID` in the denial event for traceability.

---

## Testing

`pkg/policy/dsl/dsl_test.go` covers:

- Equality match
- Not-equality
- Metadata lookup (offshore-PII rule both deny and allow branches)
- OR with parens
- NOT operator
- `contains` operator
- `startsWith` operator
- Numeric compare (`>`)
- Parse-error on truncated expression
- `AsPolicies` adapter wrapping

Run:

```bash
go test ./pkg/policy/dsl/ -v
```

---

## References

- [Google CEL](https://github.com/google/cel-go) — the production-grade alternative if scope grows
- [Open Policy Agent / Rego](https://www.openpolicyagent.org/) — another option
- [Genie's existing Go-side policies](../../pkg/governance/) — keep these for cases the DSL can't express
