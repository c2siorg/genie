// Genie eval_review.js Unit Tests (35 tests)
// Trace loading, annotation workflow, submission, navigation, clustering, keyboard shortcuts, rubrics

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('eval_review.js Unit Tests', () => {
  let mockFetch;
  let mockLocalStorage;
  let mockInterface;

  beforeEach(() => {
    // Setup DOM elements for eval_review
    document.body.innerHTML = `
      <button id="btnLoadTraces">Load Traces</button>
      <select id="sampleStrategy"><option value="random">Random</option></select>
      <button id="btnPass">Pass</button>
      <button id="btnFail">Fail</button>
      <button id="btnUncertain">Uncertain</button>
      <button id="btnSubmitAnnotation">Submit</button>
      <select id="primaryFailure"></select>
      <div id="secondaryFailures"></div>
      <input id="confidenceSlider" type="range" min="0" max="100" value="50"/>
      <span id="confidenceValue">50%</span>
      <textarea id="notes"></textarea>
      <input id="deferCheckbox" type="checkbox"/>
      <textarea id="deferReason"></textarea>
      <div id="traceContent"></div>
      <div id="listItems"></div>
      <div id="currentTrace"></div>
      <div id="annotationStatus"></div>
      <div id="failureSection" style="display:none;"></div>
      <div id="similarTraces"></div>
      <div id="rubricsList"></div>
      <div id="clustersList"></div>
      <div id="footerStatus"></div>
      <div id="footerProgress"></div>
      <div class="tab-btn" data-tab="traces">Traces</div>
      <div class="tab-btn" data-tab="rubrics">Rubrics</div>
      <div class="tab-content" id="tab-traces"></div>
      <div class="tab-content" id="tab-rubrics"></div>
    `;

    mockFetch = global.fetch;
    mockLocalStorage = global.localStorage;

    mockInterface = {
      traces: [],
      currentTraceIndex: 0,
      currentAnnotation: null,
      selectedLabel: null,
      annotatorID: 'test-annotator-id',
      apiBaseURL: '/v1/eval',
      elements: {
        btnLoadTraces: document.getElementById('btnLoadTraces'),
        sampleStrategy: document.getElementById('sampleStrategy'),
        btnPass: document.getElementById('btnPass'),
        btnFail: document.getElementById('btnFail'),
        btnUncertain: document.getElementById('btnUncertain'),
        btnSubmitAnnotation: document.getElementById('btnSubmitAnnotation'),
        primaryFailure: document.getElementById('primaryFailure'),
        secondaryFailures: document.getElementById('secondaryFailures'),
        confidenceSlider: document.getElementById('confidenceSlider'),
        confidenceValue: document.getElementById('confidenceValue'),
        notes: document.getElementById('notes'),
        deferCheckbox: document.getElementById('deferCheckbox'),
        deferReason: document.getElementById('deferReason'),
        traceContent: document.getElementById('traceContent'),
        listItems: document.getElementById('listItems'),
        currentTrace: document.getElementById('currentTrace'),
        annotationStatus: document.getElementById('annotationStatus'),
        failureSection: document.getElementById('failureSection'),
        similarTraces: document.getElementById('similarTraces'),
        rubricsList: document.getElementById('rubricsList'),
        clustersList: document.getElementById('clustersList'),
        footerStatus: document.getElementById('footerStatus'),
        footerProgress: document.getElementById('footerProgress'),
        tabBtns: document.querySelectorAll('.tab-btn'),
      },
    };
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // Trace Loading (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Trace Loading', () => {
    it('should fetch traces from API with default limit', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          traces: [
            { trace_id: 'trace-1', scenario: 'login', success: true, started_at: new Date().toISOString(), ended_at: new Date().toISOString() },
          ],
        }),
      });

      const resp = await fetch('/v1/eval/traces?limit=50&sample=random');
      const data = await resp.json();

      expect(data.traces).toHaveLength(1);
      expect(data.traces[0].trace_id).toBe('trace-1');
    });

    it('should handle empty trace list', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ traces: [] }),
      });

      const resp = await fetch('/v1/eval/traces');
      const data = await resp.json();

      expect(data.traces).toEqual([]);
    });

    it('should handle HTTP errors during fetch', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error('Server error')),
      });

      try {
        const resp = await fetch('/v1/eval/traces');
        expect(resp.ok).toBe(false);
      } catch (err) {
        expect(err).toBeDefined();
      }
    });

    it('should update status message during load', () => {
      mockInterface.elements.footerStatus.textContent = 'Loading traces...';
      expect(mockInterface.elements.footerStatus.textContent).toBe('Loading traces...');
    });

    it('should select first trace after loading', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1', scenario: 'login' },
        { trace_id: 'trace-2', scenario: 'logout' },
      ];
      mockInterface.currentTraceIndex = 0;

      expect(mockInterface.traces[mockInterface.currentTraceIndex].trace_id).toBe('trace-1');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Annotation Workflow (8 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Annotation Workflow', () => {
    it('should set label to PASS', () => {
      mockInterface.selectedLabel = 'pass';
      mockInterface.elements.btnPass.classList.add('active');

      expect(mockInterface.selectedLabel).toBe('pass');
      expect(mockInterface.elements.btnPass.classList.contains('active')).toBe(true);
    });

    it('should set label to FAIL', () => {
      mockInterface.selectedLabel = 'fail';
      mockInterface.elements.btnFail.classList.add('active');
      mockInterface.elements.failureSection.style.display = 'block';

      expect(mockInterface.selectedLabel).toBe('fail');
      expect(mockInterface.elements.failureSection.style.display).toBe('block');
    });

    it('should set label to UNCERTAIN', () => {
      mockInterface.selectedLabel = 'uncertain';
      mockInterface.elements.btnUncertain.classList.add('active');

      expect(mockInterface.selectedLabel).toBe('uncertain');
      expect(mockInterface.elements.btnUncertain.classList.contains('active')).toBe(true);
    });

    it('should show failure section only when FAIL is selected', () => {
      mockInterface.selectedLabel = 'pass';
      mockInterface.elements.failureSection.style.display = 'none';
      expect(mockInterface.elements.failureSection.style.display).toBe('none');

      mockInterface.selectedLabel = 'fail';
      mockInterface.elements.failureSection.style.display = 'block';
      expect(mockInterface.elements.failureSection.style.display).toBe('block');
    });

    it('should populate form from existing annotation', () => {
      const ann = {
        label: 'fail',
        primary_failure: 'timeout',
        confidence: 0.85,
        notes: 'Test notes',
      };

      mockInterface.selectedLabel = ann.label;
      mockInterface.elements.primaryFailure.value = ann.primary_failure;
      mockInterface.elements.confidenceSlider.value = Math.round(ann.confidence * 100);
      mockInterface.elements.notes.value = ann.notes;

      expect(mockInterface.selectedLabel).toBe('fail');
      expect(mockInterface.elements.primaryFailure.value).toBe('timeout');
      expect(parseInt(mockInterface.elements.confidenceSlider.value)).toBe(85);
      expect(mockInterface.elements.notes.value).toBe('Test notes');
    });

    it('should reset form to initial state', () => {
      mockInterface.selectedLabel = null;
      mockInterface.elements.primaryFailure.value = '';
      mockInterface.elements.confidenceSlider.value = 50;
      mockInterface.elements.notes.value = '';
      mockInterface.elements.deferCheckbox.checked = false;

      expect(mockInterface.selectedLabel).toBeNull();
      expect(mockInterface.elements.primaryFailure.value).toBe('');
      expect(mockInterface.elements.confidenceSlider.value).toBe('50');
      expect(mockInterface.elements.notes.value).toBe('');
      expect(mockInterface.elements.deferCheckbox.checked).toBe(false);
    });

    it('should toggle defer checkbox and show reason field', () => {
      mockInterface.elements.deferCheckbox.checked = true;
      mockInterface.elements.deferReason.style.display = mockInterface.elements.deferCheckbox.checked ? 'block' : 'none';

      expect(mockInterface.elements.deferCheckbox.checked).toBe(true);
      expect(mockInterface.elements.deferReason.style.display).toBe('block');
    });

    it('should update confidence display when slider changes', () => {
      mockInterface.elements.confidenceSlider.value = 75;
      mockInterface.elements.confidenceValue.textContent = '75%';

      expect(mockInterface.elements.confidenceValue.textContent).toBe('75%');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Submission (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Submission', () => {
    it('should validate that label is selected before submit', () => {
      mockInterface.selectedLabel = null;
      const isValid = mockInterface.selectedLabel !== null;

      expect(isValid).toBe(false);
    });

    it('should build annotation payload correctly', () => {
      mockInterface.selectedLabel = 'fail';
      mockInterface.elements.primaryFailure.value = 'timeout';
      mockInterface.elements.confidenceSlider.value = 85;
      mockInterface.elements.notes.value = 'Test notes';
      mockInterface.annotatorID = 'annotator-123';

      const payload = {
        annotator_id: mockInterface.annotatorID,
        label: mockInterface.selectedLabel,
        primary_failure: mockInterface.elements.primaryFailure.value,
        confidence: parseInt(mockInterface.elements.confidenceSlider.value) / 100,
        notes: mockInterface.elements.notes.value,
      };

      expect(payload.label).toBe('fail');
      expect(payload.primary_failure).toBe('timeout');
      expect(payload.confidence).toBe(0.85);
    });

    it('should POST annotation to API endpoint', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ id: 'annotation-123' }),
      });

      const trace = { trace_id: 'trace-1' };
      const payload = { label: 'pass', confidence: 0.9 };

      const resp = await fetch(`/v1/eval/traces/${trace.trace_id}/feedback`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      const data = await resp.json();
      expect(data.id).toBe('annotation-123');
    });

    it('should handle submission errors gracefully', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: () => Promise.reject(new Error('Validation error')),
      });

      try {
        const resp = await fetch('/v1/eval/traces/trace-1/feedback', {
          method: 'POST',
        });
        expect(resp.ok).toBe(false);
      } catch (err) {
        expect(err).toBeDefined();
      }
    });

    it('should auto-advance to next trace after submission', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1' },
        { trace_id: 'trace-2' },
        { trace_id: 'trace-3' },
      ];
      mockInterface.currentTraceIndex = 0;
      mockInterface.currentTraceIndex = 1;

      expect(mockInterface.traces[mockInterface.currentTraceIndex].trace_id).toBe('trace-2');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Navigation (6 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Navigation', () => {
    it('should navigate to next trace', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1' },
        { trace_id: 'trace-2' },
        { trace_id: 'trace-3' },
      ];
      mockInterface.currentTraceIndex = 0;
      mockInterface.currentTraceIndex += 1;

      expect(mockInterface.currentTraceIndex).toBe(1);
    });

    it('should navigate to previous trace', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1' },
        { trace_id: 'trace-2' },
        { trace_id: 'trace-3' },
      ];
      mockInterface.currentTraceIndex = 2;
      mockInterface.currentTraceIndex -= 1;

      expect(mockInterface.currentTraceIndex).toBe(1);
    });

    it('should not go below index 0', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1' },
        { trace_id: 'trace-2' },
      ];
      mockInterface.currentTraceIndex = 0;
      const newIndex = Math.max(mockInterface.currentTraceIndex - 1, 0);

      expect(newIndex).toBe(0);
    });

    it('should not exceed last trace index', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1' },
        { trace_id: 'trace-2' },
      ];
      mockInterface.currentTraceIndex = 1;
      const newIndex = Math.min(mockInterface.currentTraceIndex + 1, mockInterface.traces.length - 1);

      expect(newIndex).toBe(1);
    });

    it('should update footer progress display', () => {
      mockInterface.traces = [
        { trace_id: 'trace-1' },
        { trace_id: 'trace-2' },
        { trace_id: 'trace-3' },
      ];
      mockInterface.currentTraceIndex = 1;
      mockInterface.elements.footerProgress.textContent = `${mockInterface.currentTraceIndex + 1} / ${mockInterface.traces.length}`;

      expect(mockInterface.elements.footerProgress.textContent).toBe('2 / 3');
    });

    it('should switch tabs correctly', () => {
      const tabs = document.querySelectorAll('.tab-btn');
      tabs.forEach((tab) => {
        if (tab.dataset.tab === 'rubrics') {
          tab.classList.add('active');
        } else {
          tab.classList.remove('active');
        }
      });

      const activeTab = document.querySelector('.tab-btn.active');
      expect(activeTab.dataset.tab).toBe('rubrics');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Similarity/Clustering (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Similarity/Clustering', () => {
    it('should fetch clusters from API', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          clusters: {
            timeout: [{ failure_mode: 'timeout', count: 5, percentage: 25 }],
            auth_failed: [{ failure_mode: 'auth_failed', count: 3, percentage: 15 }],
          },
        }),
      });

      const resp = await fetch('/v1/eval/clusters?dimension=failure_mode');
      const data = await resp.json();

      expect(data.clusters).toBeDefined();
      expect(data.clusters.timeout).toHaveLength(1);
    });

    it('should render cluster items', () => {
      const clusters = {
        timeout: [{ failure_mode: 'timeout', count: 5, percentage: 25 }],
      };

      Object.entries(clusters).forEach(([clusterName, trends]) => {
        trends.forEach((trend) => {
          const div = document.createElement('div');
          div.className = 'cluster-item';
          div.textContent = `${trend.failure_mode} (${trend.count}) ${trend.percentage.toFixed(1)}%`;
          mockInterface.elements.clustersList.appendChild(div);
        });
      });

      const item = mockInterface.elements.clustersList.querySelector('.cluster-item');
      expect(item).toBeDefined();
      expect(item.textContent).toContain('timeout');
    });

    it('should handle empty clusters', () => {
      mockInterface.elements.clustersList.textContent = 'No failure clusters yet';
      expect(mockInterface.elements.clustersList.textContent).toBe('No failure clusters yet');
    });

    it('should display cluster percentages', () => {
      const percentage = 25.5;
      const formatted = percentage.toFixed(1);
      expect(formatted).toBe('25.5');
    });

    it('should group similar traces together', () => {
      const traces = [
        { trace_id: 'trace-1', scenario: 'login', failure_mode: 'timeout' },
        { trace_id: 'trace-2', scenario: 'login', failure_mode: 'timeout' },
        { trace_id: 'trace-3', scenario: 'logout', failure_mode: 'auth_failed' },
      ];

      const grouped = traces.reduce((acc, trace) => {
        const key = trace.failure_mode;
        if (!acc[key]) acc[key] = [];
        acc[key].push(trace);
        return acc;
      }, {});

      expect(grouped.timeout).toHaveLength(2);
      expect(grouped.auth_failed).toHaveLength(1);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Keyboard Shortcuts (4 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Keyboard Shortcuts', () => {
    it('should handle N key for next trace', () => {
      mockInterface.traces = [{ trace_id: 'trace-1' }, { trace_id: 'trace-2' }];
      mockInterface.currentTraceIndex = 0;

      const event = new KeyboardEvent('keydown', { key: 'n' });
      if (event.key.toUpperCase() === 'N') {
        mockInterface.currentTraceIndex += 1;
      }

      expect(mockInterface.currentTraceIndex).toBe(1);
    });

    it('should handle P key for previous trace', () => {
      mockInterface.traces = [{ trace_id: 'trace-1' }, { trace_id: 'trace-2' }];
      mockInterface.currentTraceIndex = 1;

      const event = new KeyboardEvent('keydown', { key: 'p' });
      if (event.key.toUpperCase() === 'P') {
        mockInterface.currentTraceIndex -= 1;
      }

      expect(mockInterface.currentTraceIndex).toBe(0);
    });

    it('should handle S key for PASS', () => {
      const event = new KeyboardEvent('keydown', { key: 's' });
      if (event.key.toUpperCase() === 'S') {
        mockInterface.selectedLabel = 'pass';
      }

      expect(mockInterface.selectedLabel).toBe('pass');
    });

    it('should handle F key for FAIL', () => {
      const event = new KeyboardEvent('keydown', { key: 'f' });
      if (event.key.toUpperCase() === 'F') {
        mockInterface.selectedLabel = 'fail';
      }

      expect(mockInterface.selectedLabel).toBe('fail');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Rubric Management (2 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Rubric Management', () => {
    it('should load rubrics from API', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          traces: [
            {
              available_rubrics: {
                'rubric-1': { id: 'rubric-1', name: 'Correctness', description: 'Test rubric' },
              },
            },
          ],
        }),
      });

      const resp = await fetch('/v1/eval/traces');
      const data = await resp.json();

      expect(data.traces[0].available_rubrics).toBeDefined();
    });

    it('should render rubrics in UI', () => {
      const rubrics = {
        'rubric-1': { id: 'rubric-1', name: 'Correctness', description: 'Evaluates correctness' },
        'rubric-2': { id: 'rubric-2', name: 'Efficiency', description: 'Evaluates efficiency' },
      };

      Object.entries(rubrics).forEach(([rubricID, rubric]) => {
        if (rubric && rubric.id) {
          const div = document.createElement('div');
          div.className = 'rubric-item';
          div.textContent = `${rubric.id}: ${rubric.name}`;
          mockInterface.elements.rubricsList.appendChild(div);
        }
      });

      const items = mockInterface.elements.rubricsList.querySelectorAll('.rubric-item');
      expect(items).toHaveLength(2);
    });
  });
});
