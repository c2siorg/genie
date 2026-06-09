# Phase 3: Accessibility & Visual Regression Tests

Comprehensive test suite for WCAG 2.1 compliance and visual regression testing covering 100 tests across 5 categories.

## Overview

Phase 3 introduces automated accessibility and visual regression testing to ensure:
- **WCAG 2.1 Level AA Compliance** across all pages
- **Keyboard Navigation** support for all interactive elements
- **Screen Reader** compatibility
- **Visual Consistency** across browsers and viewport sizes
- **Responsive Design** at all breakpoints

**Test Coverage**: 100 tests total
- WCAG 2.1 Compliance: 15 tests
- Keyboard Navigation: 15 tests
- Screen Reader Support: 15 tests
- Visual Regression (Pages): 30 tests
- Mobile & Responsive: 25 tests

---

## Installation & Setup

### 1. Install Dependencies

```bash
cd e2e
npm install
npm install --save-dev axe-core axe-playwright
```

### 2. Setup Playwright Browsers

```bash
npm run install-browsers
```

### 3. Configure Environment

Ensure your `.env` or environment has:
```
BASE_URL=http://localhost:8080
```

---

## File Structure

```
e2e/
├── tests/
│   ├── a11y/
│   │   ├── wcag.spec.ts              # 15 WCAG 2.1 compliance tests
│   │   ├── keyboard.spec.ts          # 15 keyboard navigation tests
│   │   └── screenreader.spec.ts      # 15 screen reader support tests
│   ├── visual/
│   │   ├── pages.spec.ts             # 30 page visual regression tests
│   │   └── responsive.spec.ts        # 25 responsive design tests
│   └── [existing test files]
├── utils/
│   └── a11y.utils.ts                 # Accessibility testing utilities
├── playwright.config.ts               # Updated with a11y/visual projects
└── package.json                       # Updated with a11y/visual scripts
```

---

## Test Suites

### 1. WCAG 2.1 Compliance Tests (`wcag.spec.ts`)

**15 tests** covering critical accessibility requirements:

1. **Color Contrast Ratios** (4.5:1 minimum for AA)
   - Settlement page color contrast
   - Payment page color contrast
   - Evaluation page color contrast
   - Compliance page color contrast
   - Home page color contrast

2. **Touch Target Sizes** (48x48px minimum)
   - Settlement page button/link sizes
   - Payment page form controls
   - Evaluation page interactive elements
   - Compliance page controls

3. **Heading Hierarchy** (H1 → H2 → H3, no gaps)
   - All pages heading structure verification

4. **Form Labels** (all inputs must be labeled)
   - Payment form label associations
   - Compliance form label verification

5. **Error Message Associations**
   - Form error linking to fields
   - Error message accessibility

6. **ARIA Labels** on interactive elements
   - Button accessibility labels
   - Icon button labels
   - Invalid role detection

7. **Overall Accessibility Violations**
   - Critical violation detection
   - Serious violation detection

8. **Settlement Page Specific Tests**
   - Data table accessibility
   - Status badge distinction

9. **Payment Form Accessibility**
   - Form input labels
   - Keyboard navigation support

10. **Evaluation Dashboard Accessibility**
    - Chart alternatives
    - Metric readability

11. **Compliance Form WCAG**
    - Error handling clarity
    - Field validation

12. **Focus Indicators**
    - Visible focus on all pages
    - Keyboard focus visibility

13. **Language Declaration**
    - HTML lang attribute
    - Language specification

14. **Navigation Accessibility**
    - Keyboard navigation
    - Skip links availability

15. **Comprehensive A11y Report**
    - Full audit report generation
    - Violation and pass tracking

---

### 2. Keyboard Navigation Tests (`keyboard.spec.ts`)

**15 tests** for keyboard-only access:

1. **Logical Tab Order**
   - Settlement page tab sequence
   - Payment page tab order
   - Evaluation page section navigation
   - Logical visual flow

2. **Button Keyboard Accessibility** (all pages)
   - Tab to button
   - Button activation

3. **Form Submission via Keyboard**
   - Payment form keyboard workflow
   - Compliance form keyboard access
   - Form field keyboard population

4. **Modal Escape Key Support**
   - Modal closure with Escape
   - Focus trap verification
   - Focus cycling in modal

5. **Skip Links**
   - Skip to main content
   - Skip link keyboard access
   - Skip link visibility

6. **Space and Enter Key Support**
   - Button activation with Space
   - Link activation with Enter
   - Checkbox toggle with Space

7. **Arrow Keys in Complex Components**
   - Dropdown arrow navigation
   - Menu item arrow navigation

8. **Shift+Tab Backward Navigation**
   - Reverse tab order
   - Focus movement backward

9. **No Keyboard Traps**
   - Focus escape from elements
   - Tab through workflow

10. **Custom Keyboard Handlers**
    - Custom control accessibility
    - Role attribute verification

11. **Autofocus Handling**
    - Autofocus element positioning
    - Initial keyboard navigation

12. **Enter Key on Divs**
    - Clickable div keyboard support
    - Role attribute on clickables

13. **Form Field Navigation**
    - Field tab order
    - Logical form flow

14. **Home and End Keys**
    - List navigation support
    - Home/End key handling

15. **Comprehensive Keyboard Workflow**
    - Complete keyboard navigation path
    - Multi-element traversal

---

### 3. Screen Reader Support Tests (`screenreader.spec.ts`)

**15 tests** for screen reader compatibility:

1. **ARIA Labels Present**
   - Button ARIA labels
   - Icon button labels
   - All interactive elements labeled

2. **Form Hints Announced**
   - Field descriptions
   - Required field marking
   - Help text association

3. **Live Regions for Status**
   - Loading state announcements
   - Alert messages in live regions
   - Success message announcements

4. **Loading States Announced**
   - Page loading indicators
   - Async operation completion

5. **Form Validation Announced**
   - Error announcement
   - Field validation messages
   - Error-field association

6. **Table Headers and Structure**
   - Table header markup
   - Header scope attributes
   - Table accessibility

7. **Image Alt Text**
   - All images labeled
   - Meaningful alt text
   - Image descriptions

8. **Trace Complexity Explained**
   - Complex visualization descriptions
   - Metric explanations
   - Trace clarity

9. **Link Text Meaningful**
   - Descriptive link text
   - Avoid "click here"
   - Link context

10. **Lists Properly Marked**
    - List element usage
    - List item structure

11. **Label Association**
    - Label-input association
    - Proper form structure

12. **Fieldset and Legend**
    - Grouped form fields
    - Legend descriptions

13. **Page Title and Heading**
    - Page title descriptive
    - H1 starting page

14. **ARIA Live Regions**
    - Dynamic content updates
    - Polite/assertive regions

15. **Comprehensive Screen Reader Audit**
    - Full accessibility audit
    - Multi-feature verification

---

### 4. Visual Regression Tests - Pages (`pages.spec.ts`)

**30 tests** for visual consistency:

#### Settlement Page (5 tests)
- Default state
- Filtered state
- Expanded row state
- Modal open state
- Loading state

#### Payment Page (5 tests)
- Default state
- Focused form state
- Validation error state
- Populated form state
- Success message state

#### Evaluation Dashboard (5 tests)
- Default state
- Interactive state (chart interaction)
- Expanded metrics state
- Filtered state
- Details panel open state

#### Compliance Page (5 tests)
- Default state
- Scrolled state
- Invalid field state
- Status badge state
- Progress indicator state

#### Overall Layout (5 tests)
- Header navigation
- Sidebar/navigation
- Footer
- Main content area
- Grid/flexbox layout

#### Responsive Layout (5 tests)
- Desktop 1920x1080
- Medium 1024x768
- Tablet 768x1024
- Zoom 125%
- Zoom 200%

#### Component Consistency (3 tests, counted within above)
- Button styling
- Form input styling
- Table rendering

#### Dark Mode Support (2 tests, counted within above)
- Light mode
- Dark mode

---

### 5. Mobile & Responsive Tests (`responsive.spec.ts`)

**25 tests** for responsive design:

#### Mobile Layout 375px (8 tests)
- Settlement page mobile layout
- Payment form optimization
- Evaluation dashboard reflow
- Compliance form readability
- Navigation on mobile
- Button spacing
- No horizontal scroll
- Mobile modals

#### Tablet Layout 768px (8 tests)
- Settlement tablet layout
- Payment form tablet spacing
- Evaluation dashboard tablet view
- Compliance form multi-column
- Tablet navigation
- Full table without scroll
- Sidebar accessibility
- Input sizing

#### Touch Interactions (5 tests)
- Button touch handling
- Form input touch
- Swipe gesture
- Long press handling
- Pinch zoom gesture

#### Zoom Levels (4 tests)
- 100% zoom (normal)
- 125% zoom
- 150% zoom
- 200% zoom

---

## Running Tests

### Run All Tests

```bash
npm test
```

### Run Specific Test Suites

```bash
# WCAG compliance
npm run test:a11y:wcag

# Keyboard navigation
npm run test:a11y:keyboard

# Screen reader
npm run test:a11y:screenreader

# Visual regression
npm run test:visual:pages
npm run test:visual:responsive

# All accessibility tests
npm run test:a11y

# All visual tests
npm run test:visual
```

### Run with UI

```bash
npm run test:ui
```

### Debug Mode

```bash
npm run test:debug
```

### Headed Mode (see browser)

```bash
npm run test:headed
```

### Specific Browser

```bash
npm test -- --project chromium
npm test -- --project firefox
npm test -- --project webkit
npm test -- --project mobile-chrome
```

---

## Visual Regression Baseline Setup

### Generate Initial Baselines

```bash
# Generate all visual regression baselines
npm run test:visual:update

# Or per suite
npm run test:a11y:wcag:update
```

This creates screenshot baselines in `tests/__screenshots__/` for comparison.

### Update Baselines After Design Changes

When you intentionally change the design:

```bash
npm run test:visual:update
```

This updates all baseline screenshots for comparison.

### CI/CD Integration

In your CI pipeline, baselines should be generated on main and compared against on feature branches:

```yaml
# Generate baselines on main
npm run test:visual:update
git add tests/__screenshots__

# Compare against baselines on PR
npm run test:visual
```

---

## Accessibility Testing Utilities

File: `utils/a11y.utils.ts`

Provides utilities for accessibility testing:

```typescript
// Check for violations
const violations = await checkAccessibility(page);

// Assert no critical/serious violations
await assertAccessible(page, 'serious');

// Check color contrast
const contrast = await checkColorContrast(page);

// Check input labels
const unlabeled = await checkInputLabels(page);

// Check heading hierarchy
const headingCheck = await checkHeadingHierarchy(page);

// Check ARIA labels
const ariaCheck = await checkAriaLabels(page);

// Check touch targets
const tooSmall = await checkTouchTargets(page);

// Check error associations
const unassociated = await checkErrorAssociations(page);

// Generate full report
const report = await generateA11yReport(page);
```

---

## Expected Results & Thresholds

### WCAG 2.1 Compliance

- **Color Contrast**: ≥80% AA compliance
- **Touch Targets**: ≤2 undersized elements allowed
- **Heading Hierarchy**: Must be valid (no gaps)
- **Form Labels**: 100% of inputs labeled
- **Error Association**: All errors linked to fields
- **Critical Violations**: 0 allowed
- **Serious Violations**: ≤2 allowed

### Keyboard Navigation

- All buttons keyboard accessible
- Tab order logical and visible
- Forms submittable with keyboard
- Modals escapable with Escape
- No keyboard traps
- Skip links present and functional

### Screen Reader Support

- All interactive elements labeled
- Images have alt text
- Form hints announced
- Live regions for alerts
- Table headers present
- Links have meaningful text

### Visual Regression

- Screenshot pixel diff < 100px (configurable)
- All layouts render correctly
- No unintended style changes
- Consistent across browsers
- Responsive breakpoints working

### Mobile & Responsive

- No horizontal scroll at 375px
- Touch targets ≥44px on mobile
- Proper layout at 375px, 768px, 1024px
- Text readable at 200% zoom
- Touch gestures working
- Orientation changes handled

---

## Configuration

### Playwright Config Changes

Added projects for a11y/visual testing:

```typescript
{
  name: 'a11y-chromium',
  use: { ...devices['Desktop Chrome'], colorScheme: 'light' },
},
{
  name: 'visual-chromium',
  use: { ...devices['Desktop Chrome'], screenshot: 'on' },
},
{
  name: 'mobile-visual',
  use: { ...devices['Pixel 5'], screenshot: 'on' },
},
{
  name: 'tablet-visual',
  use: { ...devices['iPad Pro'], screenshot: 'on' },
},
```

Added visual regression config:
```typescript
use: {
  maxDiffPixels: 100,
}
```

### Package.json Scripts

Added new test scripts:
```json
"test:a11y": "playwright test a11y/",
"test:a11y:wcag": "playwright test a11y/wcag.spec.ts",
"test:a11y:keyboard": "playwright test a11y/keyboard.spec.ts",
"test:a11y:screenreader": "playwright test a11y/screenreader.spec.ts",
"test:visual": "playwright test visual/",
"test:visual:pages": "playwright test visual/pages.spec.ts",
"test:visual:responsive": "playwright test visual/responsive.spec.ts",
"test:a11y:wcag:update": "playwright test a11y/wcag.spec.ts --update-snapshots",
"test:visual:update": "playwright test visual/ --update-snapshots"
```

---

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Accessibility & Visual Tests

on: [pull_request, push]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      
      - name: Install dependencies
        run: cd e2e && npm install
      
      - name: Install browsers
        run: cd e2e && npm run install-browsers
      
      - name: Start server
        run: go run ./cmd/api &
      
      - name: Wait for server
        run: sleep 5
      
      - name: Run WCAG tests
        run: cd e2e && npm run test:a11y:wcag
      
      - name: Run keyboard tests
        run: cd e2e && npm run test:a11y:keyboard
      
      - name: Run screen reader tests
        run: cd e2e && npm run test:a11y:screenreader
      
      - name: Run visual tests
        run: cd e2e && npm run test:visual
      
      - name: Upload reports
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: playwright-report
          path: e2e/playwright-report/
```

---

## Troubleshooting

### Tests Timing Out

Increase timeout in playwright.config.ts:
```typescript
timeout: 60_000, // 60 seconds
```

### Visual Baseline Mismatches

When intentional design changes occur:
```bash
npm run test:visual:update
```

### Accessibility Checks Too Strict

Adjust minImpact filter:
```typescript
const violations = await checkAccessibility(page, {
  minImpact: 'critical', // Only check critical
  ignoreRules: ['rule-id'], // Ignore specific rules
});
```

### Axe-core Not Found

Reinstall dependencies:
```bash
npm install --save-dev axe-core axe-playwright
```

### Screenshots Not Matching

Check for:
- Different font rendering across systems
- Timing issues (use `waitForLoadState('networkidle')`)
- Color management differences between browsers

---

## Best Practices

1. **Run Before Commits**: Ensure all tests pass before pushing
   ```bash
   npm test -- a11y/ visual/
   ```

2. **Keep Baselines Updated**: After design changes, update screenshots
   ```bash
   npm run test:visual:update
   ```

3. **Test Multiple Browsers**: Run on all project browsers
   ```bash
   npm run test:parallel
   ```

4. **Check Console Errors**: Monitor for JavaScript errors
   ```typescript
   page.on('console', msg => console.log(msg.text()));
   ```

5. **Use Headed Mode During Development**: See what's being tested
   ```bash
   npm run test:headed -- a11y/
   ```

6. **Review Failure Details**: Check HTML report for details
   ```bash
   npm run show:report
   ```

---

## Coverage Summary

| Category | Tests | Status |
|----------|-------|--------|
| WCAG 2.1 Compliance | 15 | ✓ Ready |
| Keyboard Navigation | 15 | ✓ Ready |
| Screen Reader Support | 15 | ✓ Ready |
| Visual Regression | 30 | ✓ Ready |
| Mobile & Responsive | 25 | ✓ Ready |
| **TOTAL** | **100** | ✓ Ready |

---

## Related Files

- [WCAG 2.1 Tests](./a11y/wcag.spec.ts) - Color contrast, heading hierarchy, form labels
- [Keyboard Tests](./a11y/keyboard.spec.ts) - Tab order, keyboard submission
- [Screen Reader Tests](./a11y/screenreader.spec.ts) - ARIA labels, live regions
- [Page Visual Tests](./visual/pages.spec.ts) - Page consistency
- [Responsive Tests](./visual/responsive.spec.ts) - Mobile, tablet, zoom
- [A11y Utilities](../utils/a11y.utils.ts) - Testing helpers

---

**Phase 3 Status**: ✅ Complete  
**Last Updated**: June 6, 2026  
**Maintainer**: Genie Team
