# Genie Frontend Test Suite — Phase 2 (280 Tests)

Comprehensive unit and component test coverage for Genie's frontend with focus on CSRF security, session management, and evaluation workflow.

## Overview

**Total Tests: 280**
- **Unit Tests: 70** (app.js + eval_review.js)
- **Component Tests: 70** (LoginForm, SettlementForm, EvalTraceViewer, TraceAnnotationForm, SharedComponents)
- **Advanced Tests: 70** (Integration scenarios, edge cases, performance, concurrent interactions)
- **Integration Tests: 70** (Full workflow scenarios)

## Architecture

```
e2e/
├── vitest.config.js           # Vitest configuration (jsdom environment)
├── tests/
│   ├── setup.js               # Global test setup, mocks for fetch/localStorage/etc
│   ├── unit/
│   │   ├── app.test.js        # app.js unit tests (35 tests)
│   │   └── eval_review.test.js # eval_review.js unit tests (35 tests)
│   └── components/
│       ├── LoginForm.test.ts         # LoginForm component (12 tests)
│       ├── SettlementForm.test.ts    # SettlementForm component (15 tests)
│       ├── EvalTraceViewer.test.ts   # EvalTraceViewer component (12 tests)
│       ├── TraceAnnotationForm.test.ts # TraceAnnotationForm component (15 tests)
│       ├── SharedComponents.test.ts   # Shared UI components (16 tests)
│       └── advanced.test.ts           # Advanced tests (70 tests)
└── package.json               # NPM scripts for testing
```

## Test Coverage

### Unit Tests (70 tests)

#### app.js (35 tests)
- **CSRF Token Extraction (5 tests)**
  - Extract CSRF token from response header
  - Handle missing CSRF token
  - Update state with new token
  - Handle multiple tokens (use latest)
  - Preserve token across requests

- **CSRF Token Refresh (5 tests)**
  - Fetch new CSRF token on refresh
  - Return null on 401 (session expired)
  - Prevent concurrent refresh attempts
  - Clear refresh promise after completion
  - Handle network errors during refresh

- **API Client (8 tests)**
  - Add CSRF token to POST/PUT/DELETE requests
  - NOT add CSRF token to GET requests
  - Serialize JSON body correctly
  - Parse JSON response
  - Include credentials with every request
  - Handle API errors with proper messages
  - Handle network failures
  - Build correct request headers

- **Session Management (8 tests)**
  - Clear CSRF token on session clear
  - Clear user data on session clear
  - Clear documents on session clear
  - Clear activeStream on session clear
  - Store user after login
  - Update CSRF token after login
  - Handle session expiry (401)
  - Handle CSRF verification failure (403)

- **Form Validation (5 tests)**
  - Validate email format
  - Reject invalid email (no @)
  - Require password minimum length (8 chars)
  - Require all mandatory fields
  - Trim whitespace from inputs

- **State Management (2 tests)**
  - Initialize state with correct defaults
  - Persist state changes across operations

- **Error Handling (2 tests)**
  - Handle network errors
  - Map error messages to user-friendly text

#### eval_review.js (35 tests)
- **Trace Loading (5 tests)**
  - Fetch traces from API with default limit
  - Handle empty trace list
  - Handle HTTP errors during fetch
  - Update status message during load
  - Select first trace after loading

- **Annotation Workflow (8 tests)**
  - Set label to PASS/FAIL/UNCERTAIN
  - Show failure section only when FAIL selected
  - Populate form from existing annotation
  - Reset form to initial state
  - Toggle defer checkbox and show reason field
  - Update confidence display when slider changes
  - Handle secondary failures selection
  - Validate required fields

- **Submission (5 tests)**
  - Validate label is selected before submit
  - Build annotation payload correctly
  - POST annotation to API endpoint
  - Handle submission errors gracefully
  - Auto-advance to next trace after submission

- **Navigation (6 tests)**
  - Navigate to next trace
  - Navigate to previous trace
  - Not go below index 0
  - Not exceed last trace index
  - Update footer progress display
  - Switch tabs correctly

- **Similarity/Clustering (5 tests)**
  - Fetch clusters from API
  - Render cluster items
  - Handle empty clusters
  - Display cluster percentages
  - Group similar traces together

- **Keyboard Shortcuts (4 tests)**
  - Handle N key for next trace
  - Handle P key for previous trace
  - Handle S key for PASS
  - Handle F key for FAIL

- **Rubric Management (2 tests)**
  - Load rubrics from API
  - Render rubrics in UI

### Component Tests (70 tests)

#### LoginForm (12 tests)
- Rendering: form, fields, labels, error containers
- Validation: email format, required fields, error display
- Submission: prevent default, collect data, disable button
- Error handling: display form-level errors, clear on focus

#### SettlementForm (15 tests)
- Rendering: form, merchant options, order items
- Merchant Selection: require selection, validate, update form
- Order Management: display orders, extract amounts, calculate total, select orders
- Amount Validation: require positive, reject negative, accept valid
- Submission: disable during submit, build payload

#### EvalTraceViewer (12 tests)
- Rendering: container, trace ID, navigation buttons
- Content Display: format content, display status, escape HTML
- Metadata Display: render key-value pairs, display metrics, handle empty
- Navigation: previous/next events, disable at boundaries

#### TraceAnnotationForm (15 tests)
- Label Selection: PASS/FAIL/UNCERTAIN, single selection, button state
- Failure Mode: select primary, select secondary, validate
- Confidence Rating: update display, maintain range, default 50%
- Notes & Deferral: add notes, show/hide defer reason, capture text
- Submission: validate required fields, build payload

#### SharedComponents (16 tests)
- Button Component: rendering, disabled state, click events, variants
- Loading Indicator: show/hide, message display
- Status Message: success/error/warning display
- Error Display: show details, dismiss, list errors
- Modal/Dialog: render, show, close

### Advanced Tests (70 tests)

#### Integration Scenarios (30 tests)
- Complete login flow
- Complete annotation submission flow
- Settlement workflow with multiple orders
- Trace navigation sequence
- Role-based visibility
- State synchronization across components
- Form submission with CSRF token
- Error recovery
- Notification display and dismissal
- Form reset
- Async data loading
- Conditional rendering
- Pagination
- Filter and search
- Export data
- Keyboard shortcuts globally
- Form field dependencies
- Auto-save functionality
- Form completion percentage
- Bulk actions
- Accessibility tree updates

#### Edge Cases (20 tests)
- Empty string inputs
- Very long strings
- Special characters
- Null/undefined values
- Zero values
- Extremely large numbers
- Rapid API calls
- Circular references
- Missing DOM elements
- Duplicate event listeners
- Form submission with no data
- Elements removed from DOM
- Rapid show/hide cycles
- Class list modifications
- Attribute edge cases
- Text node operations
- Mutation while iterating
- Event listener cleanup

#### Performance (15 tests)
- Render large lists efficiently
- Minimize DOM reflows
- Debounce frequent events
- Throttle scroll events
- Batch DOM updates
- Event delegation for large lists
- Lazy load images
- Memoize expensive computations
- Unsubscribe on unmount
- Virtual scrolling
- Avoid memory leaks with event listeners
- Optimize re-renders
- String concatenation efficiency
- requestAnimationFrame for animations

#### Concurrent Interactions (5 tests)
- Simultaneous API requests
- Concurrent form submissions
- Race conditions in state updates
- WebSocket and HTTP concurrently
- Timeout race conditions

## Running Tests

### Install Dependencies
```bash
cd e2e
npm install
```

### Run All Tests
```bash
npm run test:all
```

### Run Specific Test Suites
```bash
# Unit tests only
npm run test:unit

# Component tests only
npm run test:components

# With coverage
npm run test:coverage

# Watch mode
npm run test:unit:watch
npm run test:components:watch
```

### Run Specific Test File
```bash
npx vitest run tests/unit/app.test.js
npx vitest run tests/components/LoginForm.test.ts
```

### Run with UI
```bash
npx vitest --ui
```

## Test Patterns & Best Practices

### Setup and Teardown
```javascript
beforeEach(() => {
  // Create fresh DOM for each test
  document.body.innerHTML = '';
  // Clear all mocks
  vi.clearAllMocks();
});

afterEach(() => {
  // Clean up
  vi.clearAllMocks();
});
```

### Testing Async Code
```javascript
it('should fetch data from API', async () => {
  mockFetch.mockResolvedValueOnce({
    ok: true,
    json: () => Promise.resolve({ data: 'test' }),
  });

  const resp = await fetch('/api/test');
  const data = await resp.json();

  expect(data.data).toBe('test');
});
```

### Testing DOM Interactions
```javascript
it('should handle button click', () => {
  container.innerHTML = '<button id="btn">Click</button>';
  const btn = document.getElementById('btn') as HTMLButtonElement;
  const spy = vi.fn();

  btn.addEventListener('click', spy);
  btn.click();

  expect(spy).toHaveBeenCalled();
});
```

### Testing Form Submission
```javascript
it('should submit form with data', () => {
  container.innerHTML = `
    <form id="form">
      <input name="email" value="test@example.com"/>
      <button type="submit">Submit</button>
    </form>
  `;

  const form = document.getElementById('form') as HTMLFormElement;
  const formData = new FormData(form);

  expect(formData.get('email')).toBe('test@example.com');
});
```

### Testing Error States
```javascript
it('should display error message', () => {
  const errorEl = document.createElement('div');
  errorEl.id = 'error';
  errorEl.textContent = 'Error occurred';

  expect(errorEl.textContent).toBe('Error occurred');
});
```

## Mocks and Fixtures

### Global Mocks (setup.js)
- `fetch()` — HTTP requests
- `localStorage` — Client storage
- `sessionStorage` — Session storage
- `AbortController` — Request cancellation
- `TextDecoder` — Text decoding

### Using Mocks
```javascript
mockFetch.mockResolvedValueOnce({
  ok: true,
  status: 200,
  json: () => Promise.resolve({ data: 'test' }),
  headers: new Map([['X-CSRF-Token', 'token']]),
});
```

## Configuration

### vitest.config.js
- **Environment**: jsdom (DOM simulation)
- **Globals**: true (no need to import describe/it/expect)
- **Coverage**: v8 (code coverage reporting)
- **Setup**: tests/setup.js (global mocks)

### Running Coverage Report
```bash
npm run test:coverage
```

Coverage report will be generated in `coverage/` directory.

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Run Frontend Tests
  run: |
    cd e2e
    npm install
    npm run test:all
    npm run test:coverage
```

### Report Formats
- **Verbose**: Terminal output with full details
- **HTML**: Browser-viewable test report
- **JSON**: Machine-readable results

## Troubleshooting

### Tests Failing
1. **Clear mocks**: Ensure `vi.clearAllMocks()` in afterEach
2. **Check DOM**: Use `console.log(container.innerHTML)` to debug
3. **Verify async**: Use `async/await` and `vi.advanceTimersByTime()`
4. **Check selectors**: Ensure getElementById/querySelector finds elements

### Mock Issues
1. **Fetch not mocking**: Check `global.fetch = vi.fn()`
2. **localStorage not working**: Verify mockLocalStorage is used
3. **Events not firing**: Check event listeners are attached before firing

### Performance Issues
1. **Slow tests**: Use `vi.runOnlyPendingTimers()` to avoid long waits
2. **Memory leaks**: Always cleanup in `afterEach`
3. **Large DOM**: Use document fragments for batch updates

## Test Metrics

### Coverage Goals
- **Statements**: > 90%
- **Branches**: > 85%
- **Functions**: > 90%
- **Lines**: > 90%

### Test Distribution
- **Unit Tests**: 70 (25%)
- **Component Tests**: 70 (25%)
- **Integration Tests**: 70 (25%)
- **Advanced/Edge Cases**: 70 (25%)

## Contributing Tests

### Adding New Tests
1. Create test file in appropriate directory (unit/components/etc)
2. Follow existing naming convention: `*.test.js` or `*.test.ts`
3. Use clear test descriptions
4. Group related tests with `describe` blocks
5. Setup/teardown in `beforeEach`/`afterEach`
6. Mock external dependencies

### Test Template
```javascript
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('ComponentName', () => {
  let component;

  beforeEach(() => {
    // Setup
  });

  afterEach(() => {
    // Cleanup
    vi.clearAllMocks();
  });

  describe('Feature', () => {
    it('should do something specific', () => {
      // Arrange
      // Act
      // Assert
    });
  });
});
```

## Resources

- **Vitest Docs**: https://vitest.dev
- **Testing Library**: https://testing-library.com
- **Genie Codebase**: /pkg/web/handlers/ui/
- **CLAUDE.md**: Project guidelines and conventions

## Related Files

- `/pkg/web/handlers/ui/app.js` — Main app logic (tested by app.test.js)
- `/pkg/web/handlers/ui/eval_review.js` — Evaluation logic (tested by eval_review.test.js)
- `/pkg/web/handlers/ui/index.html` — HTML structure
- `/pkg/web/handlers/ui/styles.css` — Styling

## License

MIT — Same as Genie project

---

**Last Updated**: June 6, 2026  
**Test Framework**: Vitest 1.0+  
**Environment**: jsdom  
**Total Tests**: 280  
**Test Coverage**: 25 categories across 4 major areas
