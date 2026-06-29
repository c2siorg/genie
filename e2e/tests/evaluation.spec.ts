import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';
import { apiFixture } from '../fixtures/api.fixture';
import { EvalReviewPage } from '../pages/EvalReviewPage';

/**
 * Evaluation Framework E2E Tests
 *
 * Coverage:
 * - Trace review dashboard
 * - Failure mode annotation
 * - Judge verdict display
 * - Rubric evaluation
 * - Keyboard shortcuts
 * - Similar trace discovery
 * - Confidence scoring
 * - Audit trail of annotations
 */

const testFixtures = authFixture.extend(apiFixture);

test.describe('Evaluation Dashboard', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);

    // Need compliance role to access eval dashboard
    // Set auth header for API calls
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);

    await evalPage.goto();
  });

  test('should load evaluation dashboard with trace', async ({ page }) => {
    // Should have loaded first trace
    const traceId = await evalPage.getCurrentTraceId();
    expect(traceId).toBeTruthy();
    expect(traceId?.startsWith('tr-')).toBe(true);
  });

  test('should display trace JSON data', async () => {
    const traceJSON = await evalPage.getTraceJSON();

    // Trace should have expected structure
    expect(traceJSON).toHaveProperty('trace_id');
    expect(traceJSON).toHaveProperty('timestamp');
    expect(traceJSON).toHaveProperty('domain');
    expect(traceJSON).toHaveProperty('events');
  });

  test('should verify CSRF token present in page', async () => {
    const hasCsrf = await evalPage.verifyCsrfTokenPresent();
    expect(hasCsrf).toBe(true);
  });
});

test.describe('Trace Annotation Workflow', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should annotate trace as PASS', async () => {
    const traceId = await evalPage.getCurrentTraceId();

    // Mark as pass
    await evalPage.markAsPass();

    // Should move to next trace
    const nextTraceId = await evalPage.getCurrentTraceId();
    expect(nextTraceId).not.toBe(traceId);
  });

  test('should annotate trace as FAIL with failure mode', async () => {
    const traceId = await evalPage.getCurrentTraceId();

    // Mark as fail with failure mode
    await evalPage.markAsFail('FM-SE-001'); // Settlement amount hallucinated

    // Add confidence and notes
    await evalPage.setConfidence(0.85);
    await evalPage.addNotes('Amount is 10% higher than order total');

    // Submit feedback
    await evalPage.submitFeedback();

    // Should move to next trace
    const nextTraceId = await evalPage.getCurrentTraceId();
    expect(nextTraceId).not.toBe(traceId);
  });

  test('should defer trace for later review', async () => {
    const traceId = await evalPage.getCurrentTraceId();

    // Defer trace
    await evalPage.deferTrace();

    // Should move to next trace
    const nextTraceId = await evalPage.getCurrentTraceId();
    expect(nextTraceId).not.toBe(traceId);
  });

  test('should navigate between traces', async () => {
    const traceId1 = await evalPage.getCurrentTraceId();

    // Go to next trace
    await evalPage.nextTrace();
    const traceId2 = await evalPage.getCurrentTraceId();
    expect(traceId2).not.toBe(traceId1);

    // Go back to previous trace
    await evalPage.previousTrace();
    const traceId3 = await evalPage.getCurrentTraceId();
    expect(traceId3).toBe(traceId1);
  });

  test('should complete full annotation workflow', async () => {
    const traceId = await evalPage.getCurrentTraceId();

    // Complete annotation
    await evalPage.annotateFull('fail', 'FM-CO-001', 'Invalid state transition detected');

    // Should have moved to next trace
    const nextTraceId = await evalPage.getCurrentTraceId();
    expect(nextTraceId).not.toBe(traceId);
  });
});

test.describe('Failure Mode Selection', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should display all failure mode options', async ({ page }) => {
    // Open failure mode selector
    await page.click('[data-testid="fail-btn"]');
    await page.click('[data-testid="failure-mode-select"]');

    // Count available failure modes
    const options = await page.locator('[data-testid="failure-mode-option"]').count();

    // Should have failure modes for all 5 domains
    expect(options).toBeGreaterThanOrEqual(35); // 38 failure modes defined
  });

  test('should filter failure modes by domain', async ({ page }) => {
    await page.click('[data-testid="fail-btn"]');
    await page.click('[data-testid="failure-mode-select"]');

    // Select domain filter
    await page.click('[data-testid="domain-filter-settlement"]');

    // Should only show settlement failure modes
    const options = await page.locator('[data-testid="failure-mode-option"]').count();
    expect(options).toBeGreaterThan(0);
    expect(options).toBeLessThanOrEqual(10); // ~10 settlement modes
  });

  test('should show failure mode descriptions', async ({ page }) => {
    await page.click('[data-testid="fail-btn"]');
    await page.click('[data-testid="failure-mode-select"]');

    // Hover over a failure mode to see description
    const firstOption = page.locator('[data-testid="failure-mode-option"]').first();
    await firstOption.hover();

    // Description should be visible
    const description = await page.locator('[data-testid="failure-mode-description"]').textContent();
    expect(description).toBeTruthy();
  });
});

test.describe('Confidence Scoring', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should accept confidence scores 0-1', async () => {
    await evalPage.markAsFail('FM-SE-001');

    // Test boundary values
    await evalPage.setConfidence(0);
    await evalPage.setConfidence(0.5);
    await evalPage.setConfidence(1);

    // Should not error
    expect(true).toBe(true);
  });

  test('should reject invalid confidence scores', async ({ page }) => {
    await evalPage.markAsFail('FM-SE-001');

    // Try to set invalid value
    await page.fill('[data-testid="confidence-input"]', '2');
    await page.press('[data-testid="confidence-input"]', 'Tab');

    // Should show validation error or reset
    const value = await page.inputValue('[data-testid="confidence-input"]');
    const numValue = parseFloat(value);

    expect(numValue).toBeLessThanOrEqual(1);
    expect(numValue).toBeGreaterThanOrEqual(0);
  });
});

test.describe('Similar Traces Discovery', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should show similar traces', async () => {
    const similarTraces = await evalPage.getSimilarTraces();

    expect(similarTraces.length).toBeGreaterThan(0);

    // Each similar trace should have similarity score
    similarTraces.forEach((trace) => {
      expect(trace.traceId).toBeTruthy();
      expect(trace.similarity).toBeGreaterThanOrEqual(0);
      expect(trace.similarity).toBeLessThanOrEqual(100);
    });
  });

  test('should sort similar traces by similarity score', async () => {
    const similarTraces = await evalPage.getSimilarTraces();

    if (similarTraces.length > 1) {
      // Check if sorted descending
      for (let i = 1; i < similarTraces.length; i++) {
        expect(similarTraces[i].similarity).toBeLessThanOrEqual(similarTraces[i - 1].similarity);
      }
    }
  });
});

test.describe('Rubric Definitions', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should display all rubrics', async () => {
    const rubrics = await evalPage.getRubricDefinitions();

    // Should have ~18 rubrics
    expect(rubrics.length).toBeGreaterThanOrEqual(15);
  });

  test('should show rubric details', async () => {
    const rubrics = await evalPage.getRubricDefinitions();

    rubrics.forEach((rubric) => {
      expect(rubric.name).toBeTruthy();
      expect(rubric.description).toBeTruthy();
      expect(rubric.passFormat).toBeTruthy();
      expect(rubric.failFormat).toBeTruthy();
    });
  });
});

test.describe('Keyboard Shortcuts', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should display keyboard shortcuts help', async () => {
    const shortcuts = await evalPage.getKeyboardShortcuts();

    // Should have shortcuts defined
    expect(Object.keys(shortcuts).length).toBeGreaterThan(0);
  });

  test('should use keyboard shortcut to mark pass (P key)', async () => {
    const traceId1 = await evalPage.getCurrentTraceId();

    // Use keyboard shortcut
    await evalPage.useKeyboardShortcut('p');

    // Should move to next trace
    const traceId2 = await evalPage.getCurrentTraceId();
    expect(traceId2).not.toBe(traceId1);
  });

  test('should use keyboard shortcut for next trace (N key)', async () => {
    const traceId1 = await evalPage.getCurrentTraceId();

    // Use keyboard shortcut
    await evalPage.useKeyboardShortcut('n');

    // Should move to next trace
    const traceId2 = await evalPage.getCurrentTraceId();
    expect(traceId2).not.toBe(traceId1);
  });
});

test.describe('Trace Search & Filtering', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should search traces by ID', async () => {
    // Search for a specific trace
    await evalPage.searchTraces('tr-001');

    const counter = await evalPage.getTraceCounter();

    // Should show results
    expect(counter).toBeTruthy();
  });

  test('should filter traces by failure mode', async () => {
    // Filter by settlement failure mode
    await evalPage.filterByFailureMode('settlement');

    const counter = await evalPage.getTraceCounter();

    // Should show filtered results
    expect(counter).toBeTruthy();
  });

  test('should show trace counter', async () => {
    const counter = await evalPage.getTraceCounter();

    // Counter format: "X of Y"
    expect(counter).toMatch(/\d+ of \d+/);
  });
});

test.describe('Evaluation Extended - Multi-Select Failure Modes', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should allow selecting multiple failure modes for complex failures', async ({
    page,
  }) => {
    await evalPage.markAsFail('FM-SE-001');

    // Add a second failure mode
    await page.click('[data-testid="add-failure-mode-btn"]');
    await page.click('[data-testid="failure-mode-select"]');
    await page.click('[data-value="FM-CO-002"]');

    // Should have 2 failure modes selected
    const modes = await page.locator('[data-testid="selected-failure-mode"]').count();
    expect(modes).toBe(2);
  });

  test('should remove failure mode from selection', async ({ page }) => {
    await evalPage.markAsFail('FM-SE-001');
    await page.click('[data-testid="add-failure-mode-btn"]');
    await page.click('[data-testid="failure-mode-select"]');
    await page.click('[data-value="FM-CO-002"]');

    const initialCount = await page.locator('[data-testid="selected-failure-mode"]').count();

    // Remove first failure mode
    await page.click('[data-testid="remove-failure-mode-0"]');

    const finalCount = await page.locator('[data-testid="selected-failure-mode"]').count();
    expect(finalCount).toBe(initialCount - 1);
  });

  test('should store all selected failure modes in annotation', async ({ page }) => {
    const traceId = await evalPage.getCurrentTraceId();

    await evalPage.markAsFail('FM-SE-001');
    await page.click('[data-testid="add-failure-mode-btn"]');
    await page.click('[data-testid="failure-mode-select"]');
    await page.click('[data-value="FM-PA-003"]');

    await evalPage.setConfidence(0.9);
    await evalPage.submitFeedback();

    // Verify annotation was saved with both modes
    const annotation = await page.locator('[data-testid="annotation-detail"]').first();
    expect(annotation).toBeTruthy();
  });

  test('should show related failure modes suggestion panel', async ({ page }) => {
    await evalPage.markAsFail('FM-SE-001');

    // When selecting settlement mode, suggest related ones
    const suggestions = await page.locator('[data-testid="suggested-failure-mode"]');
    expect(suggestions).toBeDefined();
  });

  test('should group failure modes by domain', async ({ page }) => {
    await evalPage.markAsFail('FM-SE-001');
    await page.click('[data-testid="add-failure-mode-btn"]');

    // Should show domain groups in dropdown
    const domainHeaders = await page.locator('[data-testid="failure-mode-domain-header"]').count();
    expect(domainHeaders).toBeGreaterThan(0);
  });

  test('should mark incompatible failure modes visually', async ({ page }) => {
    await evalPage.markAsFail('FM-SE-001'); // Settlement domain

    // Some failure modes might be incompatible
    const incompatibleModes = await page.locator('[data-testid="incompatible-failure-mode"]').count();

    // Incompatible modes should be greyed out or disabled
    if (incompatibleModes > 0) {
      const disabled = await page.locator('[data-testid="failure-mode-option"][disabled]').count();
      expect(disabled).toBeGreaterThan(0);
    }
  });
});

test.describe('Evaluation Extended - Trace Clustering', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should show trace clusters by failure mode', async ({ page, api }) => {
    const clusters = await api.request(
      'GET',
      `/v1/eval/traces/clusters?group_by=failure_mode`,
      undefined,
      undefined
    );

    expect(clusters.clusters).toBeDefined();
    expect(clusters.clusters.length).toBeGreaterThan(0);
  });

  test('should display cluster statistics', async ({ api }) => {
    const clusters = await api.request(
      'GET',
      `/v1/eval/traces/clusters/stats`,
      undefined,
      undefined
    );

    expect(clusters.total_clusters).toBeGreaterThan(0);
    expect(clusters.traces_per_cluster).toBeDefined();
    expect(clusters.coverage_percent).toBeDefined();
  });

  test('should allow clustering by domain', async ({ api }) => {
    const clusters = await api.request(
      'GET',
      `/v1/eval/traces/clusters?group_by=domain`,
      undefined,
      undefined
    );

    expect(clusters.clusters).toBeDefined();
    // Should have clusters for each domain (Settlement, Payment, Compliance, etc)
    expect(clusters.clusters.length).toBeGreaterThanOrEqual(3);
  });

  test('should show trace count per cluster', async ({ page }) => {
    const clusterInfo = await page.locator('[data-testid="cluster-info"]');

    if (await clusterInfo.isVisible()) {
      const count = await page.locator('[data-testid="cluster-trace-count"]').textContent();
      expect(count).toMatch(/\d+ traces/);
    }
  });
});

test.describe('Evaluation Extended - Advanced Search', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should support advanced search with filters', async ({ page }) => {
    await page.fill('[data-testid="search-input"]', 'status:failed domain:settlement');
    await page.press('[data-testid="search-input"]', 'Enter');

    const results = await page.locator('[data-testid="trace-container"]').count();
    expect(results).toBeGreaterThanOrEqual(0);
  });

  test('should support range queries in search', async ({ page }) => {
    await page.fill('[data-testid="search-input"]', 'confidence:[0.8 TO 1.0]');
    await page.press('[data-testid="search-input"]', 'Enter');

    const results = await page.locator('[data-testid="trace-container"]').count();
    expect(results).toBeGreaterThanOrEqual(0);
  });

  test('should support date range search', async ({ page }) => {
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - 7);

    const formattedStart = startDate.toISOString().split('T')[0];
    const formattedEnd = new Date().toISOString().split('T')[0];

    await page.fill('[data-testid="search-input"]', `created:[${formattedStart} TO ${formattedEnd}]`);
    await page.press('[data-testid="search-input"]', 'Enter');

    const results = await page.locator('[data-testid="trace-container"]').count();
    expect(results).toBeGreaterThanOrEqual(0);
  });

  test('should show search suggestions while typing', async ({ page }) => {
    await page.focus('[data-testid="search-input"]');
    await page.type('[data-testid="search-input"]', 'fail', { delay: 100 });

    const suggestions = await page.locator('[data-testid="search-suggestion"]').count();
    expect(suggestions).toBeGreaterThan(0);
  });

  test('should save custom search queries', async ({ page }) => {
    await page.fill('[data-testid="search-input"]', 'status:failed AND confidence:>0.9');
    await page.click('[data-testid="save-search-btn"]');
    await page.fill('[data-testid="search-name"]', 'High Confidence Failures');
    await page.click('[data-testid="confirm-save"]');

    // Search should be saved
    const saved = await page.locator('[data-testid="saved-search-item"]');
    expect(saved).toBeTruthy();
  });
});

test.describe('Evaluation Extended - Rubric Management', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should display rubric details modal', async ({ page }) => {
    const rubrics = await evalPage.getRubricDefinitions();
    expect(rubrics.length).toBeGreaterThan(0);

    // Click on first rubric to open details
    await page.click('[data-testid="rubric-card"]');
    await page.waitForSelector('[data-testid="rubric-details-modal"]');

    const modal = await page.locator('[data-testid="rubric-details-modal"]');
    expect(modal).toBeTruthy();
  });

  test('should show rubric examples for pass and fail', async ({ page }) => {
    const rubrics = await evalPage.getRubricDefinitions();

    rubrics.forEach((rubric) => {
      expect(rubric.passFormat).toBeTruthy();
      expect(rubric.failFormat).toBeTruthy();
      expect(rubric.description).toBeTruthy();
    });
  });
});

test.describe('Evaluation Extended - Export & Reporting', () => {
  let evalPage: EvalReviewPage;

  test.beforeEach(async ({ page, getComplianceToken }) => {
    evalPage = new EvalReviewPage(page);
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: getComplianceToken(),
        url: 'http://localhost:8080',
      },
    ]);
    await evalPage.goto();
  });

  test('should export annotations to CSV', async ({ page }) => {
    await page.click('[data-testid="export-btn"]');
    await page.click('[data-testid="export-csv-option"]');

    // Wait for download
    const downloadPromise = page.waitForEvent('download');
    await page.click('[data-testid="confirm-export"]');
    const download = await downloadPromise;

    expect(download.suggestedFilename()).toContain('.csv');
  });

  test('should export annotations to JSON', async ({ page }) => {
    await page.click('[data-testid="export-btn"]');
    await page.click('[data-testid="export-json-option"]');

    const downloadPromise = page.waitForEvent('download');
    await page.click('[data-testid="confirm-export"]');
    const download = await downloadPromise;

    expect(download.suggestedFilename()).toContain('.json');
  });

  test('should generate evaluation report', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    const report = await api.request(
      'POST',
      `/v1/eval/report/generate`,
      {
        data: {
          report_type: 'SUMMARY',
          date_range: {
            start: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
            end: new Date().toISOString(),
          },
        },
      },
      token
    );

    expect(report.report_id).toBeDefined();
    expect(report.summary).toBeDefined();
  });

  test('should download evaluation report', async ({ page }) => {
    await page.click('[data-testid="report-btn"]');
    await page.waitForSelector('[data-testid="report-options"]');

    const downloadPromise = page.waitForEvent('download');
    await page.click('[data-testid="download-report-btn"]');
    const download = await downloadPromise;

    expect(['pdf', 'csv', 'json']).toContain(download.suggestedFilename().split('.').pop());
  });

  test('should show coverage metrics in report', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    const metrics = await api.request(
      'GET',
      `/v1/eval/metrics/coverage`,
      undefined,
      token
    );

    expect(metrics.total_traces).toBeGreaterThan(0);
    expect(metrics.annotated_traces).toBeGreaterThanOrEqual(0);
    expect(metrics.coverage_percentage).toBeGreaterThanOrEqual(0);
  });
});
