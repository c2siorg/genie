# Phase 1: Manual Testing Guide

**Purpose**: Verify Phase 1 UI/UX improvements work correctly across all user interactions.

**Time Required**: 30-45 minutes

**Prerequisites**: 
- Server running on `http://localhost:8080`
- Modern browser (Chrome, Firefox, Safari, Edge)
- Screen reader (for a11y tests): NVDA (Windows), JAWS (Windows), VoiceOver (Mac)
- Keyboard only (no mouse)

---

## Test Categories

### 1. ARIA Labels & Semantic HTML (10 min)

#### Test 1.1: Form Input Labels
**Steps**:
1. Open DevTools (F12)
2. Inspect each form input: email, password, name, CSV file input
3. Right-click → Inspect → look for `aria-label` attribute

**Verify**:
- [ ] Email input has `aria-label="Email address"`
- [ ] Password input has `aria-label="Password"`
- [ ] CSV file input has `aria-label="Select CSV file to upload"`
- [ ] Each input has `aria-describedby` linking to help text

**Screen Reader Test**:
- Activate screen reader
- Tab through form fields
- Verify screen reader announces each field's purpose and hint text

---

#### Test 1.2: Tab Navigation Roles
**Steps**:
1. Open DevTools
2. Find the main navigation tabs (Ask, Documents, Governance, Settings)
3. Inspect the `<nav>` and `<button>` elements

**Verify**:
- [ ] `<nav>` has `role="tablist"`
- [ ] Each tab button has `role="tab"`
- [ ] Active tab has `aria-selected="true"`
- [ ] Inactive tabs have `aria-selected="false"`
- [ ] Active tab has `tabindex="0"`
- [ ] Inactive tabs have `tabindex="-1"`

---

#### Test 1.3: Error Containers
**Steps**:
1. Try submitting login form with empty email
2. Open DevTools
3. Inspect error message container

**Verify**:
- [ ] Error container has `role="alert"`
- [ ] Error container has `aria-live="polite"`
- [ ] Error text is visible and red
- [ ] Error is associated with the form via `aria-describedby`

---

#### Test 1.4: Table Accessibility
**Steps**:
1. Navigate to Documents tab
2. Open DevTools
3. Inspect the documents table

**Verify**:
- [ ] Table has a `<caption class="sr-only">` describing its purpose
- [ ] Each column header has `scope="col"`
- [ ] Screen reader can navigate table cells with arrow keys

---

### 2. Keyboard Navigation (15 min)

#### Test 2.1: Tab Navigation with Keyboard
**Steps**:
1. Reload page
2. Press **Right Arrow** key repeatedly
3. Verify tab switches (Ask → Documents → Governance → Settings → Ask)

**Verify**:
- [ ] Tab changes on each Right Arrow press
- [ ] Tab wraps from Settings back to Ask
- [ ] Currently active tab is visually highlighted

#### Test 2.2: Tab Navigation with Home/End Keys
**Steps**:
1. Click on the "Documents" tab
2. Press **Home** key
3. Verify you're now on "Ask" tab
4. Press **End** key
5. Verify you're now on "Settings" tab

**Verify**:
- [ ] Home key jumps to first tab (Ask)
- [ ] End key jumps to last tab (Settings)

---

#### Test 2.3: Escape Key
**Steps**:
1. Click "Ask" tab
2. Ask a question and click "Ask (stream)"
3. Wait for response to start
4. Press **Escape** key
5. Verify streaming stops

**Verify**:
- [ ] Escape key stops active streaming
- [ ] Response is cancelled (no error)

**Alternative**: Click report card to open it, press Escape to close it

---

#### Test 2.4: Form Navigation
**Steps**:
1. Click "Sign in" tab in auth card
2. Press **Tab** key repeatedly to navigate form fields
3. Verify order: Email → Password → Submit button
4. Press **Escape** to unfocus

**Verify**:
- [ ] Tab moves focus to next field (visible blue outline)
- [ ] Shift+Tab moves focus to previous field
- [ ] All form inputs have visible `:focus-visible` outline (2px purple)
- [ ] Submit button is reachable via Tab

---

### 3. Form Validation & Error Display (10 min)

#### Test 3.1: Login Form Validation
**Steps**:
1. Navigate to Sign in tab
2. Leave email and password empty
3. Click "Sign in" button

**Verify**:
- [ ] No `alert()` dialog appears
- [ ] Email field shows red error message below it
- [ ] Password field shows red error message below it
- [ ] Error messages are specific ("Email is required", "Password is required")
- [ ] Email field has red border

---

#### Test 3.2: Signup Form Validation
**Steps**:
1. Click "Sign up" tab
2. Enter: Name="John", Email="invalid-email", Password="short"
3. Click "Create account"

**Verify**:
- [ ] Email field error: "Invalid email format"
- [ ] Password field error: "Password must be at least 8 characters"
- [ ] Form is not submitted (stays on signup form)

---

#### Test 3.3: Error Clearing on Focus
**Steps**:
1. Submit form with empty email (see error message)
2. Click on email field to focus it
3. Verify error disappears

**Verify**:
- [ ] Error message disappears when field is focused
- [ ] Red border is removed
- [ ] `aria-invalid="true"` is removed from input

---

#### Test 3.4: Loading State
**Steps**:
1. Fill login form with valid email/password
2. Click "Sign in"
3. Observe button during loading

**Verify**:
- [ ] Button shows spinner (rotating circle)
- [ ] Button text changes to show loading state
- [ ] Button is disabled (not clickable)
- [ ] Button text returns and spinner stops when done

---

### 4. Loading Skeletons (8 min)

#### Test 4.1: Skeleton Animation
**Steps**:
1. Login successfully
2. Navigate to Documents tab
3. Observe table while it loads

**Verify**:
- [ ] 2 skeleton rows appear in table
- [ ] Skeleton rows have shimmer animation (gradient moving left to right)
- [ ] Animation loops smoothly every 1.5 seconds

---

#### Test 4.2: Skeleton Replacement
**Steps**:
1. Wait for documents to load
2. Observe skeleton rows disappear

**Verify**:
- [ ] Skeleton rows fade out
- [ ] Real document rows replace skeletons
- [ ] No visual flicker or jarring transition

---

### 5. Focus Indicators (5 min)

#### Test 5.1: Focus Visibility on Buttons
**Steps**:
1. Press **Tab** key to navigate to a button
2. Look for focus indicator

**Verify**:
- [ ] Button has visible 2px outline when focused
- [ ] Outline color is distinct (purple/accent color)
- [ ] Outline has 2px offset from button

---

#### Test 5.2: Focus Visibility on Tabs
**Steps**:
1. Press **Tab** several times to reach a tab button
2. Look for focus indicator

**Verify**:
- [ ] Tab button has visible 2px outline when focused
- [ ] Outline is clearly visible (not hidden by tab styling)

---

#### Test 5.3: Mouse vs Keyboard Focus
**Steps**:
1. Click a button with mouse
2. Look for focus outline (should not show on click)
3. Press Tab to move to a button
4. Look for focus outline (should show on keyboard focus)

**Verify**:
- [ ] Focus outline shows only on keyboard focus (`:focus-visible`)
- [ ] No outline when clicking with mouse (`:focus` without `:focus-visible`)

---

### 6. Live Regions (5 min)

#### Test 6.1: Error Announcements
**Steps**:
1. Activate screen reader
2. Try to submit form with empty email
3. Listen to screen reader announcement

**Verify**:
- [ ] Screen reader announces error when it appears
- [ ] Announcement uses `aria-live="polite"` (waits for natural pause)
- [ ] Error is announced without interrupting user

---

#### Test 6.2: Status Updates
**Steps**:
1. Click "Ask" button to submit a question
2. Listen to screen reader while response arrives

**Verify**:
- [ ] Screen reader announces loading status
- [ ] Status updates are announced as response streams

---

### 7. Accessibility Verification Script (5 min)

#### Test 7.1: Run Automated Verification
**Steps**:
1. Open terminal
2. Run: `./pkg/web/handlers/ui/verify-a11y.sh`
3. Observe output

**Verify**:
- [ ] Script runs without errors
- [ ] Output shows accessibility score (target: ≥8.5/10)
- [ ] JSON report is generated at `pkg/web/handlers/ui/a11y-report.json`

#### Test 7.2: Check JSON Report
**Steps**:
1. Open generated `a11y-report.json`
2. Review metrics

**Verify**:
- [ ] `aria_label_count` ≥ 50
- [ ] `aria_describedby_count` ≥ 5
- [ ] `aria_live_regions` ≥ 5
- [ ] `semantic_roles_found` ≥ 7
- [ ] `keyboard_navigation` is true
- [ ] `css_focus_visible` is true
- [ ] Overall score ≥ 8.5/10

---

### 8. Cross-Browser Testing (5 min)

#### Test 8.1: Chrome/Chromium
**Steps**:
1. Open page in Chrome
2. Run through tests 1-6

**Verify**:
- [ ] All tests pass in Chrome

#### Test 8.2: Firefox
**Steps**:
1. Open page in Firefox
2. Run through tests 1-6 (at least 2-3 critical tests)

**Verify**:
- [ ] Critical tests pass in Firefox

#### Test 8.3: Safari (if Mac)
**Steps**:
1. Open page in Safari
2. Run through tests 1-6 (at least 2-3 critical tests)

**Verify**:
- [ ] Critical tests pass in Safari

---

## 📊 Test Results Scorecard

| Category | Test | Status | Notes |
|----------|------|--------|-------|
| ARIA Labels | 1.1 Form Inputs | [ ] ✓ | |
| | 1.2 Tab Roles | [ ] ✓ | |
| | 1.3 Error Containers | [ ] ✓ | |
| | 1.4 Table a11y | [ ] ✓ | |
| Keyboard Nav | 2.1 Tab Keys | [ ] ✓ | |
| | 2.2 Home/End | [ ] ✓ | |
| | 2.3 Escape | [ ] ✓ | |
| | 2.4 Form Nav | [ ] ✓ | |
| Form Validation | 3.1 Login Errors | [ ] ✓ | |
| | 3.2 Signup Errors | [ ] ✓ | |
| | 3.3 Error Clearing | [ ] ✓ | |
| | 3.4 Loading State | [ ] ✓ | |
| Skeletons | 4.1 Animation | [ ] ✓ | |
| | 4.2 Replacement | [ ] ✓ | |
| Focus | 5.1 Button Focus | [ ] ✓ | |
| | 5.2 Tab Focus | [ ] ✓ | |
| | 5.3 Keyboard vs Mouse | [ ] ✓ | |
| Live Regions | 6.1 Error Announce | [ ] ✓ | |
| | 6.2 Status Updates | [ ] ✓ | |
| A11y Script | 7.1 Run Script | [ ] ✓ | Score: ____ |
| | 7.2 Check JSON | [ ] ✓ | |
| Browser Compat | 8.1 Chrome | [ ] ✓ | |
| | 8.2 Firefox | [ ] ✓ | |
| | 8.3 Safari | [ ] ✓ | |

---

## ✅ Sign-Off

**Tested by**: ________________  
**Date**: ________________  
**Browser(s)**: ________________  
**Overall Result**: [ ] PASS [ ] FAIL  
**Issues Found**: ________________

**Notes**:
```
[Space for notes on any issues or observations]
```

---

## 📚 Reference

- **Accessibility Guide**: `A11Y_VERIFICATION.md`
- **Automated Verification**: `./pkg/web/handlers/ui/verify-a11y.sh`
- **Code Changes**: `PHASE1_DEPLOYMENT_SUMMARY.md`
- **Project Status**: `SESSION_SUMMARY.md`

---

**Last Updated**: June 1, 2026  
**Phase**: 1 (UI/UX Improvements)  
**Status**: ✅ Complete
