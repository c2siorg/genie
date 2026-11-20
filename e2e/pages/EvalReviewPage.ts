import { Page } from '@playwright/test';

/**
 * Evaluation Review Page Object
 * Encapsulates evaluation dashboard & trace annotation UI
 */
export class EvalReviewPage {
  constructor(private page: Page) {}

  // Selectors
  private readonly TRACE_CONTAINER = '[data-testid="trace-container"]';
  private readonly TRACE_JSON = '[data-testid="trace-json"]';
  private readonly PASS_BTN = '[data-testid="pass-btn"]';
  private readonly FAIL_BTN = '[data-testid="fail-btn"]';
  private readonly DEFER_BTN = '[data-testid="defer-btn"]';
  private readonly FAILURE_MODE_SELECT = '[data-testid="failure-mode-select"]';
  private readonly CONFIDENCE_INPUT = '[data-testid="confidence-input"]';
  private readonly NOTES_TEXTAREA = '[data-testid="notes-textarea"]';
  private readonly SUBMIT_BTN = '[data-testid="submit-feedback-btn"]';
  private readonly SIMILAR_TRACES = '[data-testid="similar-traces"]';
  private readonly RUBRIC_CARD = '[data-testid="rubric-card"]';
  private readonly NEXT_TRACE_BTN = '[data-testid="next-trace-btn"]';
  private readonly PREV_TRACE_BTN = '[data-testid="prev-trace-btn"]';
  private readonly TRACE_COUNTER = '[data-testid="trace-counter"]';
  private readonly SEARCH_INPUT = '[data-testid="search-input"]';
  private readonly FILTER_DROPDOWN = '[data-testid="filter-dropdown"]';

  /**
   * Navigate to eval review dashboard
   */
  async goto() {
    await this.page.goto('/eval/traces');
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Get current trace ID
   */
  async getCurrentTraceId(): Promise<string> {
    const traceId = await this.page.locator('[data-testid="current-trace-id"]').textContent();
    return traceId?.trim() || '';
  }

  /**
   * Get full trace JSON
   */
  async getTraceJSON(): Promise<any> {
    const jsonText = await this.page.locator(this.TRACE_JSON).textContent();
    return JSON.parse(jsonText || '{}');
  }

  /**
   * Mark trace as PASS
   */
  async markAsPass() {
    await this.page.click(this.PASS_BTN);
    await this.page.waitForSelector(this.NEXT_TRACE_BTN, { state: 'visible' });
  }

  /**
   * Mark trace as FAIL and select failure mode
   */
  async markAsFail(failureMode: string) {
    await this.page.click(this.FAIL_BTN);
    await this.page.click(this.FAILURE_MODE_SELECT);
    await this.page.click(`[data-value="${failureMode}"]`);
    await this.page.waitForSelector(this.CONFIDENCE_INPUT, { state: 'visible' });
  }

  /**
   * Set confidence level
   */
  async setConfidence(confidence: number) {
    await this.page.fill(this.CONFIDENCE_INPUT, confidence.toString());
  }

  /**
   * Add annotation notes
   */
  async addNotes(notes: string) {
    await this.page.fill(this.NOTES_TEXTAREA, notes);
  }

  /**
   * Submit feedback
   */
  async submitFeedback() {
    await this.page.click(this.SUBMIT_BTN);
    // Wait for submission to complete
    await this.page.waitForURL(/.*\/traces\/.+/);
  }

  /**
   * Complete full annotation workflow
   */
  async annotateFull(verdict: 'pass' | 'fail', failureMode?: string, notes?: string) {
    if (verdict === 'pass') {
      await this.markAsPass();
    } else if (verdict === 'fail' && failureMode) {
      await this.markAsFail(failureMode);
      if (notes) {
        await this.addNotes(notes);
      }
      await this.setConfidence(0.9);
      await this.submitFeedback();
    }
  }

  /**
   * Defer trace for later
   */
  async deferTrace() {
    await this.page.click(this.DEFER_BTN);
    await this.page.waitForSelector(this.NEXT_TRACE_BTN, { state: 'visible' });
  }

  /**
   * Go to next trace
   */
  async nextTrace() {
    await this.page.click(this.NEXT_TRACE_BTN);
    await this.page.waitForSelector(this.TRACE_CONTAINER, { state: 'visible' });
  }

  /**
   * Go to previous trace
   */
  async previousTrace() {
    await this.page.click(this.PREV_TRACE_BTN);
    await this.page.waitForSelector(this.TRACE_CONTAINER, { state: 'visible' });
  }

  /**
   * Get similar traces
   */
  async getSimilarTraces() {
    const count = await this.page.locator(`${this.SIMILAR_TRACES} [data-testid="similar-trace-item"]`).count();
    const traces = [];

    for (let i = 0; i < count; i++) {
      const item = this.page.locator(`${this.SIMILAR_TRACES} [data-testid="similar-trace-item"]`).nth(i);
      const traceId = await item.locator('[data-testid="trace-id"]').textContent();
      const similarity = await item.locator('[data-testid="similarity-score"]').textContent();

      traces.push({
        traceId: traceId?.trim(),
        similarity: parseFloat(similarity?.replace('%', '') || '0'),
      });
    }

    return traces;
  }

  /**
   * Get rubric definitions
   */
  async getRubricDefinitions() {
    const count = await this.page.locator(this.RUBRIC_CARD).count();
    const rubrics = [];

    for (let i = 0; i < count; i++) {
      const card = this.page.locator(this.RUBRIC_CARD).nth(i);
      const name = await card.locator('[data-testid="rubric-name"]').textContent();
      const description = await card.locator('[data-testid="rubric-description"]').textContent();
      const passFormat = await card.locator('[data-testid="pass-format"]').textContent();
      const failFormat = await card.locator('[data-testid="fail-format"]').textContent();

      rubrics.push({
        name: name?.trim(),
        description: description?.trim(),
        passFormat: passFormat?.trim(),
        failFormat: failFormat?.trim(),
      });
    }

    return rubrics;
  }

  /**
   * Search for traces
   */
  async searchTraces(query: string) {
    await this.page.fill(this.SEARCH_INPUT, query);
    await this.page.press(this.SEARCH_INPUT, 'Enter');
    await this.page.waitForSelector(this.TRACE_CONTAINER, { state: 'visible' });
  }

  /**
   * Filter by failure mode
   */
  async filterByFailureMode(failureMode: string) {
    await this.page.click(this.FILTER_DROPDOWN);
    await this.page.click(`[data-filter-value="${failureMode}"]`);
    await this.page.waitForSelector(this.TRACE_CONTAINER, { state: 'visible' });
  }

  /**
   * Get trace counter (e.g., "3 of 50")
   */
  async getTraceCounter(): Promise<string> {
    const counter = await this.page.textContent(this.TRACE_COUNTER);
    return counter?.trim() || '';
  }

  /**
   * Verify CSRF token is present in page
   */
  async verifyCsrfTokenPresent(): Promise<boolean> {
    const csrfToken = await this.page.locator('[name="csrf_token"]').inputValue();
    return typeof csrfToken === 'string' && csrfToken.length > 0;
  }

  /**
   * Get keyboard shortcuts help
   */
  async getKeyboardShortcuts() {
    const shortcuts: Record<string, string> = {};

    const entries = await this.page.locator('[data-testid="keyboard-shortcut"]').count();
    for (let i = 0; i < entries; i++) {
      const entry = this.page.locator('[data-testid="keyboard-shortcut"]').nth(i);
      const key = await entry.locator('[data-testid="shortcut-key"]').textContent();
      const action = await entry.locator('[data-testid="shortcut-action"]').textContent();

      if (key && action) {
        shortcuts[key.trim()] = action.trim();
      }
    }

    return shortcuts;
  }

  /**
   * Use keyboard shortcut
   */
  async useKeyboardShortcut(key: string) {
    await this.page.press('body', key);
    await this.page.waitForTimeout(500); // Wait for action to complete
  }
}
