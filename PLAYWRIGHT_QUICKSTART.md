# Playwright E2E Tests — Quick Start Guide

## Installation (One-Time Setup)

```bash
make playwright-install
# This installs dependencies and browser binaries
```

Or manually:
```bash
cd e2e
npm install
npx playwright install
```

## Start Services

Before running tests, you need a running Genie API stack:

```bash
# Terminal 1: Start services
docker-compose up -d

# Verify health (wait for "healthy" status)
docker-compose ps

# View logs if needed
docker-compose logs genie-api
```

## Run Tests

### All Tests (82 tests)
```bash
make playwright-test
```

### By Test Suite

```bash
# Settlement workflow (18 tests)
make playwright-test-settlement

# Security & CSRF (22 tests)
make playwright-test-security

# Evaluation dashboard (22 tests)
make playwright-test-evaluation

# Compliance & AML (20 tests)
make playwright-test-compliance
```

### By Browser

```bash
cd e2e && npm run test:chrome     # Chromium only
cd e2e && npm run test:firefox    # Firefox only
cd e2e && npm run test:webkit     # Safari/WebKit only
cd e2e && npm run test:mobile     # Mobile Chrome
```

### Interactive Modes

```bash
# UI Mode (visual control panel)
make playwright-test-ui

# Debug Mode (Playwright Inspector)
make playwright-test-debug

# Headed Mode (visible browser)
cd e2e && npm run test:headed
```

### Record Test Code

Interactively record test code as you interact with the app:

```bash
make playwright-codegen
# Opens browser, records your interactions as Playwright code
```

## View Reports

After tests run, view the HTML report:

```bash
make playwright-report
# Opens report in browser
```

Reports include:
- Test results (pass/fail)
- Execution time
- Screenshots of failures
- Video recordings (on failure)
- Traces (for debugging)

## Common Workflows

### Run Settlement Tests in UI Mode
```bash
make playwright-test-ui
# Click "Settlement" to run those tests
```

### Debug a Failing Test
```bash
make playwright-test-debug
# Opens Playwright Inspector
# Step through test, inspect DOM, check console
```

### Run Single Test
```bash
cd e2e
npx playwright test settlement.spec.ts -g "should verify amount correctness"
```

### Run with Specific Browser
```bash
cd e2e
npx playwright test settlement.spec.ts --project firefox
```

### Record Video of Test Run
Videos are auto-recorded on failure. To force recording:

Edit `playwright.config.ts`:
```typescript
use: {
  video: 'on',  // Change from 'retain-on-failure'
}
```

### Check Test Coverage by File
```bash
cd e2e && npm test 2>&1 | grep -E "✓|×"
```

## Troubleshooting

### "Port 8080 already in use"
```bash
# Find and kill process using port 8080
lsof -i :8080
kill -9 <PID>
```

### "Database connection refused"
```bash
# Restart database
docker-compose restart db
docker-compose logs db
```

### "Playwright: browser not installed"
```bash
npx playwright install
# Or for specific browser:
npx playwright install chromium
```

### "Timeout waiting for selector"
- Check if API is running: `curl http://localhost:8080/health`
- Increase timeout in `playwright.config.ts`: `timeout: 60_000`
- View test video to see what happened: `make playwright-report`

### Tests Pass Locally But Fail in CI
- Ensure Docker environment matches CI exactly
- Check environment variables (GENIE_LLM, GENIE_JWT_SECRET, etc.)
- Verify timeouts are sufficient for CI resources
- Check test results/videos in CI artifacts

## Configuration

### Environment Variables

Set in `.env` or docker-compose.yml:

```bash
BASE_URL=http://localhost:8080          # API endpoint
GENIE_LLM=mock                          # mock | ollama | openai
GENIE_OLLAMA_URL=http://ollama:11434   # For Ollama backend
GENIE_JWT_SECRET=test-secret-key...     # JWT signing key
GENIE_KEK_BASE64=...                    # Encryption key
```

### Playwright Config

Edit `e2e/playwright.config.ts`:

```typescript
timeout: 30_000,                    // 30s per test
workers: 4,                         // Parallel workers
retries: 0,                         // Retries per test
use: {
  baseURL: 'http://localhost:8080',
  screenshot: 'only-on-failure',
  video: 'retain-on-failure',
  trace: 'on-first-retry',
}
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Install Playwright
  run: make playwright-install

- name: Start Services
  run: docker-compose up -d

- name: Wait for Health
  run: |
    for i in {1..30}; do
      curl -f http://localhost:8080/health && break
      sleep 2
    done

- name: Run E2E Tests
  run: make playwright-test-ci

- name: Upload Report
  uses: actions/upload-artifact@v3
  if: always()
  with:
    name: playwright-report
    path: e2e/playwright-report/
```

### Local CI Equivalent

```bash
make playwright-install
docker-compose up -d
# Wait for health
sleep 10
make playwright-test-ci
make playwright-report
```

## Performance Tips

1. **Run tests in parallel** (default 4 workers):
   ```bash
   make playwright-test
   ```

2. **Use headless mode** (default):
   - Faster than headed mode
   - Good for CI/local runs

3. **Disable videos for faster runs**:
   ```typescript
   // In playwright.config.ts
   video: 'off',  // or 'retain-on-failure'
   ```

4. **Use serial mode for debugging**:
   ```bash
   cd e2e && npm run test:serial
   ```

## Key Files

| File | Purpose |
|------|---------|
| `e2e/playwright.config.ts` | Test configuration |
| `e2e/docker-compose.yml` | Local test environment |
| `e2e/package.json` | Dependencies & scripts |
| `e2e/fixtures/` | Reusable test helpers |
| `e2e/pages/` | Page objects (UI encapsulation) |
| `e2e/tests/` | Test suites |
| `e2e/README.md` | Comprehensive guide |
| `E2E_TESTS_SUMMARY.md` | This project's E2E overview |

## Next Steps

1. ✅ Install: `make playwright-install`
2. ✅ Start services: `docker-compose up -d`
3. ✅ Run tests: `make playwright-test`
4. ✅ View report: `make playwright-report`
5. ✅ Integrate with CI/CD

## Support

- **Playwright Docs**: https://playwright.dev
- **E2E README**: `e2e/README.md`
- **Summary**: `E2E_TESTS_SUMMARY.md`

---

**Last Updated**: June 6, 2026  
**Test Count**: 82 tests  
**Coverage**: Settlement, Security, Evaluation, Compliance  
**Status**: ✅ Ready to run
