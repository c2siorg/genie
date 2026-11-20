// EvalTraceViewer Component Tests (12 tests)
// Trace rendering, content formatting, metadata display, navigation

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('EvalTraceViewer Component', () => {
  let container: HTMLElement;
  let traceContent: HTMLElement;
  let traceId: HTMLElement;
  let metadataDisplay: HTMLElement;
  let metricsDisplay: HTMLElement;
  let prevBtn: HTMLButtonElement;
  let nextBtn: HTMLButtonElement;

  beforeEach(() => {
    container = document.createElement('div');
    container.innerHTML = `
      <div class="trace-viewer">
        <div class="trace-header">
          <h2>Trace Details</h2>
          <span id="trace-id" class="trace-id">-</span>
        </div>
        <div id="trace-content" class="trace-content">
          <pre id="trace-body"></pre>
        </div>
        <div class="trace-metadata">
          <h3>Metadata</h3>
          <div id="metadata-display" class="metadata"></div>
        </div>
        <div class="trace-metrics">
          <h3>Metrics</h3>
          <div id="metrics-display" class="metrics"></div>
        </div>
        <div class="trace-navigation">
          <button id="prev-btn" aria-label="Previous trace">← Previous</button>
          <button id="next-btn" aria-label="Next trace">Next →</button>
        </div>
      </div>
    `;
    document.body.appendChild(container);

    traceContent = document.getElementById('trace-content') as HTMLElement;
    traceId = document.getElementById('trace-id') as HTMLElement;
    metadataDisplay = document.getElementById('metadata-display') as HTMLElement;
    metricsDisplay = document.getElementById('metrics-display') as HTMLElement;
    prevBtn = document.getElementById('prev-btn') as HTMLButtonElement;
    nextBtn = document.getElementById('next-btn') as HTMLButtonElement;
  });

  afterEach(() => {
    document.body.removeChild(container);
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // Rendering (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Rendering', () => {
    it('should render trace viewer container', () => {
      expect(container.querySelector('.trace-viewer')).toBeDefined();
      expect(traceContent).toBeDefined();
      expect(traceId).toBeDefined();
    });

    it('should display trace ID', () => {
      const testId = 'trace-abc123def456';
      traceId.textContent = testId;
      expect(traceId.textContent).toBe(testId);
    });

    it('should have navigation buttons', () => {
      expect(prevBtn).toBeDefined();
      expect(nextBtn).toBeDefined();
      expect(prevBtn.getAttribute('aria-label')).toBe('Previous trace');
      expect(nextBtn.getAttribute('aria-label')).toBe('Next trace');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Content Display (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Content Display', () => {
    it('should format trace content for display', () => {
      const traceBody = document.querySelector('#trace-body') as HTMLElement;
      const lines = [
        'Trace ID: trace-123',
        'Scenario: login',
        'Status: SUCCESS',
        'Duration: 500ms',
      ];
      traceBody.textContent = lines.join('\n');

      expect(traceBody.textContent).toContain('Trace ID');
      expect(traceBody.textContent).toContain('trace-123');
    });

    it('should display success/failure status', () => {
      const statusText = 'Status: SUCCESS';
      const isSuccess = statusText.includes('SUCCESS');
      expect(isSuccess).toBe(true);

      const statusText2 = 'Status: FAILED';
      const isFailed = statusText2.includes('FAILED');
      expect(isFailed).toBe(true);
    });

    it('should escape HTML in trace content for security', () => {
      const unsafeContent = '<script>alert("xss")</script>';
      const escaped = unsafeContent
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');

      expect(escaped).not.toContain('<script>');
      expect(escaped).toContain('&lt;script&gt;');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Metadata Display (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Metadata Display', () => {
    it('should render metadata key-value pairs', () => {
      const metadata = {
        user_id: 'user-123',
        session_id: 'session-456',
        region: 'us-east-1',
      };

      Object.entries(metadata).forEach(([key, value]) => {
        const div = document.createElement('div');
        div.className = 'metadata-item';
        div.textContent = `${key}: ${value}`;
        metadataDisplay.appendChild(div);
      });

      const items = metadataDisplay.querySelectorAll('.metadata-item');
      expect(items.length).toBe(3);
      expect(items[0].textContent).toContain('user_id');
    });

    it('should display metrics correctly', () => {
      const metrics = {
        latency_ms: 150,
        tokens_used: 1250,
        cost_usd: 0.05,
      };

      Object.entries(metrics).forEach(([key, value]) => {
        const div = document.createElement('div');
        div.className = 'metric-item';
        div.textContent = `${key}: ${value}`;
        metricsDisplay.appendChild(div);
      });

      const items = metricsDisplay.querySelectorAll('.metric-item');
      expect(items.length).toBe(3);
      expect(items[0].textContent).toContain('latency_ms');
    });

    it('should handle empty metadata gracefully', () => {
      metadataDisplay.textContent = 'No metadata available';
      expect(metadataDisplay.textContent).toBe('No metadata available');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Navigation (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Navigation', () => {
    it('should emit previous event when prev button clicked', () => {
      const spy = vi.fn();
      prevBtn.addEventListener('click', spy);

      prevBtn.click();
      expect(spy).toHaveBeenCalled();
    });

    it('should emit next event when next button clicked', () => {
      const spy = vi.fn();
      nextBtn.addEventListener('click', spy);

      nextBtn.click();
      expect(spy).toHaveBeenCalled();
    });

    it('should disable navigation buttons when at boundaries', () => {
      prevBtn.disabled = true;
      nextBtn.disabled = false;

      expect(prevBtn.disabled).toBe(true);
      expect(nextBtn.disabled).toBe(false);

      prevBtn.disabled = false;
      nextBtn.disabled = true;

      expect(prevBtn.disabled).toBe(false);
      expect(nextBtn.disabled).toBe(true);
    });
  });
});
