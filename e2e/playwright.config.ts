import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright Configuration for Genie E2E Tests
 *
 * Covers:
 * - Payment & Settlement workflows (Phase 2)
 * - Security features (CSRF, HttpOnly cookies)
 * - Evaluation dashboard & annotation
 * - Compliance & AML workflows
 * - Multi-agent orchestration
 */
export default defineConfig({
  testDir: './tests',
  testMatch: '**/*.spec.ts',
  testIgnore: '**/node_modules/**',

  // Run tests in parallel (4 workers by default)
  workers: process.env.CI ? 2 : 4,

  // Timeout settings
  timeout: 30_000,
  expect: { timeout: 5_000 },

  // Global timeout for all tests
  globalTimeout: 30 * 60 * 1000,

  // Output
  reporter: [
    ['html', { outputFolder: 'playwright-report' }],
    ['json', { outputFile: 'test-results.json' }],
    ['junit', { outputFile: 'junit.xml' }],
    ['list'],
  ],

  // Global configuration
  use: {
    // Base URL for API
    baseURL: process.env.BASE_URL || 'http://localhost:8080',

    // Screenshot on failure
    screenshot: 'only-on-failure',
    screenshot_on_failure: true,

    // Video on failure
    video: 'retain-on-failure',

    // Trace for debugging
    trace: 'on-first-retry',

    // User-Agent
    userAgent: 'Genie-E2E-Test/1.0',

    // Disable web security for CSRF testing
    launchArgs: ['--disable-web-security'],

    // Visual regression configuration
    maxDiffPixels: 100,
  },

  // Browser configurations
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 5'] },
    },
    // Accessibility & Visual Regression Tests (Phase 3)
    {
      name: 'a11y-chromium',
      use: {
        ...devices['Desktop Chrome'],
        colorScheme: 'light',
      },
    },
    {
      name: 'visual-chromium',
      use: {
        ...devices['Desktop Chrome'],
        screenshot: 'on',
      },
    },
    {
      name: 'mobile-visual',
      use: {
        ...devices['Pixel 5'],
        screenshot: 'on',
      },
    },
    {
      name: 'tablet-visual',
      use: {
        ...devices['iPad Pro'],
        screenshot: 'on',
      },
    },
  ],

  // Web server setup (optional - for running against live server)
  webServer: process.env.SKIP_WEB_SERVER
    ? undefined
    : {
        command: 'go run ./cmd/api',
        url: 'http://localhost:8080/health',
        reuseExistingServer: !process.env.CI,
        timeout: 120 * 1000,
      },
});
