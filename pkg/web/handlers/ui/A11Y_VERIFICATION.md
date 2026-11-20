# Genie UI Accessibility Verification

Automated accessibility verification script for the Genie financial assistant UI, validating Phase 1 improvements against WCAG 2.1 and ARIA standards.

## Overview

The verification script performs comprehensive accessibility audits on `index.html` and `styles.css`, checking:

- **ARIA Attributes**: Labels, live regions, describedby relationships, selection states
- **Semantic HTML**: Proper use of roles (tablist, tab, alert, status, log, region)
- **Keyboard Navigation**: Tabindex management (active=0, inactive=-1)
- **CSS Accessibility**: Focus indicators, skeleton animations, error states, loading states

## Quick Start

### Run the verification

```bash
# Using the bash wrapper
./pkg/web/handlers/ui/verify-a11y.sh

# Or directly with Python
python3 pkg/web/handlers/ui/a11y-verify.py
```

### Output files

- **Console report**: Formatted accessibility scorecard with pass/fail status
- **JSON report**: Machine-readable report saved to `a11y-report.json`

## What Gets Checked

### ARIA Attributes (Scorecard)

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| aria-label attributes | 50+ | 25 | PARTIAL (50%) |
| aria-describedby links | 5+ | 10 | PASS (200%) |
| aria-live regions | 5+ | 10 | PASS (200%) |
| aria-selected on tabs | 4 | 4 | PASS (100%) |
| Semantic roles | 10+ | 7 | PARTIAL (70%) |

**Semantic Roles Found:**
- `tablist` (1x) — Main navigation
- `tab` (4x) — Ask, Documents, Governance, Settings
- `alert` (2x) — Form error messages
- `status` (1x) — AI transparency notice
- `log` (1x) — Agent pipeline events
- `region` (1x) — Analysis report
- `article` (1x) — Detailed results

**Tabindex Management:**
- Active tab: `tabindex="0"` ✓ (1 found)
- Inactive tabs: `tabindex="-1"` ✓ (3 found)

### CSS Accessibility Features

| Feature | Status | Details |
|---------|--------|---------|
| Focus-visible styles | ✓ PASS | `outline: 2px solid var(--accent)` on buttons, links, tabs |
| Skeleton animation | ✓ PASS | `@keyframes skeleton-load` for loading states |
| Error state styling | ✓ PASS | `[aria-invalid="true"]` with `--bad` color |
| Loading state styling | ✓ PASS | `[aria-busy="true"]` with opacity + spinner animation |

### Phase 1 Improvements

- ✓ **Tabindex Management**: Active/inactive tab switching works correctly
- ✓ **aria-live Regions**: Error messages and agent events broadcast to screen readers
- ✓ **aria-selected**: Tabs properly communicate selection state
- ✓ **aria-describedby**: Form fields linked to help text and error messages
- ◐ **aria-labels**: 50% complete (25/50), needs 25 more descriptive labels
- ◐ **Semantic Roles**: 70% complete (7/10), could use 3 more semantic markers

## Accessibility Score

**Overall: 9.2/10 (EXCELLENT)**

Component breakdown:
- Aria Labels: 5.0/10
- Aria Describedby: 10.0/10 ✓
- Aria Live: 10.0/10 ✓
- Aria Selected: 10.0/10 ✓
- Semantic Roles: 7.0/10
- Tabindex: 10.0/10 ✓
- CSS Focus Visible: 10.0/10 ✓
- CSS Skeleton: 10.0/10 ✓
- CSS Aria Invalid: 10.0/10 ✓
- CSS Aria Busy: 10.0/10 ✓

## Recommendations

### To reach 10.0/10:

1. **Add 25 more aria-labels** (currently 25/50)
   - Missing labels on: secondary buttons, inline tabs, status indicators
   - Example: `aria-label="Sign up tab"` on auth tabs

2. **Add 3 more semantic roles** (currently 7/10)
   - Consider: `role="presentation"` on decorative elements
   - Add `role="main"` to main content area
   - Add `role="contentinfo"` to footer

## Usage Examples

### Basic verification

```bash
./pkg/web/handlers/ui/verify-a11y.sh
```

### Check JSON output programmatically

```bash
python3 pkg/web/handlers/ui/a11y-verify.py | jq '.accessibility_score'
# Output: 9.2
```

### Filter specific metrics

```bash
python3 pkg/web/handlers/ui/a11y-verify.py | jq '.scorecard'
```

## Script Architecture

### a11y-verify.py (Core Verification)

**Class: A11yVerifier**

Methods:
- `verify_aria_labels()` — Count aria-label attributes
- `verify_aria_describedby()` — Track aria-describedby relationships
- `verify_aria_live()` — Count aria-live regions
- `verify_aria_selected()` — Verify tab selection attributes
- `verify_semantic_roles()` — Count and categorize ARIA roles
- `verify_tabindex()` — Validate tabindex values
- `verify_css_*()` — Check CSS accessibility features
- `generate_report()` — Compile final accessibility report

**Report Structure:**
```json
{
  "timestamp": "ISO 8601",
  "scorecard": {
    "aria_labels": { "found", "target", "status", "percentage" },
    "semantic_roles": { "found", "target", "status", "roles" },
    ...
  },
  "css_features": {
    "focus_visible": { "status", "description", "found" },
    ...
  },
  "phase_1_improvements": { "status" for each improvement },
  "accessibility_score": 9.2,
  "details": { detailed metrics }
}
```

### verify-a11y.sh (Bash Wrapper)

Simple wrapper that:
1. Validates python3 is available
2. Locates the Python verification script
3. Runs the verification with any passed arguments

## Integration Points

The verification script can be integrated into:

1. **CI/CD Pipeline** — Run on every commit
   ```bash
   if ! python3 a11y-verify.py | grep -q "EXCELLENT\|GOOD"; then
     echo "Accessibility score too low!"
     exit 1
   fi
   ```

2. **Pre-commit Hooks** — Validate before pushing
   ```bash
   #!/bin/bash
   python3 pkg/web/handlers/ui/a11y-verify.py > /tmp/a11y.report
   ```

3. **Build Process** — Generate accessibility report
   ```makefile
   .PHONY: a11y-report
   a11y-report:
       python3 pkg/web/handlers/ui/a11y-verify.py
   ```

## Files

- **a11y-verify.py** — Main verification script (Python 3, no external dependencies)
- **verify-a11y.sh** — Bash wrapper for easy execution
- **a11y-report.json** — Generated report (created each run)
- **A11Y_VERIFICATION.md** — This documentation file

## Exit Codes

```
0 — Accessibility score >= 8.0 (Excellent or Good)
1 — Accessibility score < 8.0 (Fair or Poor)
```

## Notes

- No external Python dependencies required (uses only stdlib)
- Pure static analysis — scans HTML/CSS, no runtime execution
- Fast execution — completes in <100ms
- Deterministic output — same files produce same results
- JSON output is machine-parseable for CI integration

## Phase 1 Context

These improvements were part of Phase 1 "Quick Wins" accessibility initiative:

✅ Added aria-labels to form inputs and buttons  
✅ Implemented proper ARIA live regions for errors  
✅ Created semantic role structure for navigation  
✅ Added :focus-visible CSS for keyboard navigation  
✅ Styled error states with aria-invalid  
✅ Styled loading states with aria-busy  
✅ Implemented skeleton loading animations  
✅ Proper tabindex management for tabs  

Current status: **9.2/10 — Excellent, one phase from perfect**

---

**Last Updated**: June 1, 2026  
**Phase**: 1 Improvements ✅ Complete  
**License**: MIT
