# Phase 1: Quick QA Checklist (20 minutes)

**Quick verification for Phase 1 UI/UX improvements.**

**Time**: 20 minutes  
**Scope**: 5 critical test areas, 1 automated check  
**Automation**: 30% automated, 70% manual

---

## ⏱️ Timeline

| Phase | Duration | What to Test |
|-------|----------|--------------|
| 1 | 3 min | Setup & login |
| 2 | 4 min | Keyboard navigation |
| 3 | 3 min | Form validation & errors |
| 4 | 4 min | Loading states & skeletons |
| 5 | 3 min | Focus visibility |
| 6 | 3 min | Automated a11y check |

---

## 🚀 Phase 1: Setup (3 min)

```bash
# Terminal 1: Start server
go run ./cmd/api

# Terminal 2: Open browser
open http://localhost:8080  # Mac
# or: start http://localhost:8080  # Windows
# or: xdg-open http://localhost:8080  # Linux
```

**Checklist**:
- [ ] Page loads without errors
- [ ] Header visible with tabs (Ask, Documents, Governance, Settings)
- [ ] Auth form visible (Sign in / Sign up)

---

## ⌨️ Phase 2: Keyboard Navigation (4 min)

### 2.1: Tab Switching with Arrow Keys (2 min)

```
1. Login with any email/password
2. Press Right Arrow repeatedly → switches tabs (Ask → Documents → Governance → Settings → Ask)
3. Press Left Arrow repeatedly → switches tabs backwards
4. Press Home → jumps to Ask tab
5. Press End → jumps to Settings tab
```

**Pass Criteria**:
- [ ] Right arrow changes tabs in order
- [ ] Left arrow reverses order
- [ ] Home key goes to Ask
- [ ] End key goes to Settings
- [ ] Escape key doesn't break anything

### 2.2: Form Navigation with Tab Key (1 min)

```
1. Go to Sign in tab (before login)
2. Press Tab repeatedly → navigates Email → Password → Sign in button
3. Shift+Tab → goes backwards
4. Each focused element has purple 2px outline
```

**Pass Criteria**:
- [ ] Tab moves through fields in correct order
- [ ] Each field has visible focus outline (2px purple)
- [ ] Shift+Tab goes backwards
- [ ] Button is reachable via Tab

### 2.3: Escape Key (1 min)

```
1. Ask a question and click Ask (stream)
2. Wait for response to start
3. Press Escape
4. Response should stop
```

**Pass Criteria**:
- [ ] Escape stops streaming (no error)
- [ ] Page is still usable

---

## ✅ Phase 3: Form Validation & Errors (3 min)

### 3.1: Error Messages (1.5 min)

```
1. Try to login with empty email and password
2. Look for error messages below each field
3. Each error field has red border
```

**Pass Criteria**:
- [ ] Error appears without `alert()` dialog
- [ ] Error text is red and below field
- [ ] Email field has red border
- [ ] Error is specific ("Email is required", etc.)
- [ ] No page freeze or hang

### 3.2: Error Clearing (1 min)

```
1. With error showing, click on email field
2. Error message disappears
3. Red border disappears
```

**Pass Criteria**:
- [ ] Error clears on field focus
- [ ] Red border removed
- [ ] Field is immediately usable

### 3.3: Loading Button State (0.5 min)

```
1. Fill login form with valid email/password
2. Click Sign in
3. Button shows spinner (rotating circle)
4. Button is disabled during loading
```

**Pass Criteria**:
- [ ] Spinner appears on button
- [ ] Button is not clickable
- [ ] Spinner disappears when done

---

## 🎬 Phase 4: Loading States & Skeletons (4 min)

### 4.1: Skeleton Animation (2 min)

```
1. Login successfully
2. Click Documents tab
3. Observe 2 skeleton rows in table
4. Watch shimmer effect (gradient moving left→right)
5. Wait for real documents to load
```

**Pass Criteria**:
- [ ] Skeletons appear immediately
- [ ] Shimmer animation is smooth (1.5s loop)
- [ ] Skeletons fade out when real data appears
- [ ] No flicker or jarring transition

### 4.2: Spinner on Upload (1 min)

```
1. Click Documents tab
2. Click "Select CSV file"
3. After selecting a file, click Upload
4. Observe form becomes disabled
5. Spinner appears
```

**Pass Criteria**:
- [ ] Form fields are disabled during upload
- [ ] Button shows loading state
- [ ] Form returns to normal after upload

### 4.3: Spinner on Ask (1 min)

```
1. Click Ask tab
2. Type a question
3. Click Ask or Ask (stream)
4. Observe spinner on button
5. Observe "loading response" indication
```

**Pass Criteria**:
- [ ] Button shows spinner
- [ ] Response appears and button returns to normal
- [ ] User can see something is happening (not frozen)

---

## 👁️ Phase 5: Focus Visibility (3 min)

### 5.1: Keyboard Focus (1.5 min)

```
1. Reload page
2. Press Tab repeatedly to navigate form fields
3. Each field has visible 2px outline when focused
4. Outline color is purple (accent color)
5. Outline has offset from element
```

**Pass Criteria**:
- [ ] Focus outline visible on Tab
- [ ] Focus outline visible on Tab navigation
- [ ] No outline when clicking with mouse (keyboard-only indicator)
- [ ] Outline is distinct and easy to see

### 5.2: Tab Focus (1 min)

```
1. After login, press Tab to reach tab buttons
2. Tab button has visible purple outline
3. Currently active tab is highlighted differently
```

**Pass Criteria**:
- [ ] Tab button has focus outline
- [ ] Active tab styling is different
- [ ] Can navigate between tabs with arrow keys and keyboard

### 5.3: Input Focus Ring (0.5 min)

```
1. Press Tab to reach email/password inputs
2. Each input has visible focus outline (2px purple)
3. Outline has 2px offset
```

**Pass Criteria**:
- [ ] Input has visible focus outline
- [ ] Outline is distinct (not black, purple preferred)
- [ ] Outline doesn't overlap text

---

## 🤖 Phase 6: Automated A11y Check (3 min)

### 6.1: Run Verification Script (1.5 min)

```bash
# Terminal
./pkg/web/handlers/ui/verify-a11y.sh
```

**Expected Output**:
```
ARIA Attributes:
  aria-label: 25/50 (50%)
  aria-describedby: 10/5 (200%)
  aria-live: 10/5 (200%)
  aria-selected: 4/4 (100%)
  
Semantic Roles: 7/10 (70%)
Keyboard Navigation: PASS
CSS Features: PASS

Overall Score: 9.2/10 (EXCELLENT)
```

**Pass Criteria**:
- [ ] Script runs without errors
- [ ] Score ≥ 8.5/10
- [ ] All major categories show good coverage

### 6.2: Check JSON Report (1.5 min)

```bash
cat pkg/web/handlers/ui/a11y-report.json | jq '.'
```

**Pass Criteria**:
- [ ] `aria_label_count` ≥ 50
- [ ] `aria_live_regions` ≥ 5
- [ ] `keyboard_navigation` is true
- [ ] `css_focus_visible` is true
- [ ] Overall score is 9.2/10 or higher

---

## 📋 Results Scorecard

| Phase | Tests | Status | Notes |
|-------|-------|--------|-------|
| 1: Setup | Page load | [ ] ✓ | |
| 2: Keyboard | Tab switching | [ ] ✓ | |
| | Form nav | [ ] ✓ | |
| | Escape key | [ ] ✓ | |
| 3: Form Validation | Error display | [ ] ✓ | |
| | Error clearing | [ ] ✓ | |
| | Button state | [ ] ✓ | |
| 4: Loading States | Skeleton animation | [ ] ✓ | |
| | Upload spinner | [ ] ✓ | |
| | Ask spinner | [ ] ✓ | |
| 5: Focus Visibility | Keyboard focus | [ ] ✓ | |
| | Tab focus | [ ] ✓ | |
| | Input outline | [ ] ✓ | |
| 6: A11y Script | Script runs | [ ] ✓ | Score: __/10 |
| | JSON report | [ ] ✓ | |

---

## ✅ Sign-Off

```
QA Tester: ___________________
Date: ___________________
Browsers Tested: ___________________

Overall Result:  [ ] PASS  [ ] FAIL

Issues Found (if any):
_________________________________
_________________________________
_________________________________
```

---

## 🚨 Failure Criteria

**STOP and report if:**
- ❌ Alert dialog appears instead of inline error
- ❌ Form field doesn't get red border on error
- ❌ Button doesn't show spinner when loading
- ❌ Skeleton doesn't animate
- ❌ Tab navigation doesn't work with arrow keys
- ❌ Focus outline is invisible
- ❌ A11y script reports score < 8.5/10
- ❌ Page crashes or becomes unresponsive

---

## ✨ Success Criteria

**PASS if:**
- ✅ All keyboard navigation works smoothly
- ✅ Form errors appear inline without alerts
- ✅ Loading states show spinner + are not clickable
- ✅ Skeletons animate and fade smoothly
- ✅ Focus outline is visible on keyboard navigation
- ✅ A11y script shows score ≥ 8.5/10
- ✅ All tests complete in < 20 minutes

---

## 📚 Quick Reference

| Key | Action |
|-----|--------|
| Tab | Next field |
| Shift+Tab | Previous field |
| Arrow Right/Down | Next tab |
| Arrow Left/Up | Previous tab |
| Home | First tab |
| End | Last tab |
| Escape | Stop streaming / close modal |

---

## 📖 Links

- **Detailed Tests**: `PHASE1_TEST_GUIDE.md`
- **Code Changes**: `PHASE1_DEPLOYMENT_SUMMARY.md`
- **A11y Details**: `A11Y_VERIFICATION.md`
- **Project Status**: `SESSION_SUMMARY.md`

---

**Last Updated**: June 1, 2026  
**Duration**: 20 minutes  
**Status**: ✅ Phase 1 Complete
