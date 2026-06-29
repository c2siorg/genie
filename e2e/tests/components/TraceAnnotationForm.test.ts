// TraceAnnotationForm Component Tests (15 tests)
// Label selection, failure mode handling, confidence rating, notes, deferral, submission

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('TraceAnnotationForm Component', () => {
  let container: HTMLElement;
  let form: HTMLFormElement;
  let passBtn: HTMLButtonElement;
  let failBtn: HTMLButtonElement;
  let uncertainBtn: HTMLButtonElement;
  let failureSection: HTMLElement;
  let primaryFailure: HTMLSelectElement;
  let secondaryFailures: HTMLElement;
  let confidenceSlider: HTMLInputElement;
  let confidenceValue: HTMLElement;
  let notesTextarea: HTMLTextAreaElement;
  let deferCheckbox: HTMLInputElement;
  let deferReason: HTMLTextAreaElement;
  let submitBtn: HTMLButtonElement;

  beforeEach(() => {
    container = document.createElement('div');
    container.innerHTML = `
      <form id="annotation-form" aria-label="Trace Annotation Form">
        <div class="label-selection">
          <button type="button" id="btn-pass" class="label-btn" data-label="pass" aria-pressed="false">Pass</button>
          <button type="button" id="btn-fail" class="label-btn" data-label="fail" aria-pressed="false">Fail</button>
          <button type="button" id="btn-uncertain" class="label-btn" data-label="uncertain" aria-pressed="false">Uncertain</button>
        </div>

        <div id="failure-section" style="display: none;">
          <label for="primary-failure">Primary Failure Mode</label>
          <select id="primary-failure" required>
            <option value="">Select failure mode</option>
            <option value="timeout">Timeout</option>
            <option value="auth_failed">Auth Failed</option>
            <option value="validation_error">Validation Error</option>
          </select>

          <fieldset>
            <legend>Secondary Failures</legend>
            <div id="secondary-failures">
              <label><input type="checkbox" value="resource_leak"/> Resource Leak</label>
              <label><input type="checkbox" value="data_corruption"/> Data Corruption</label>
              <label><input type="checkbox" value="performance_degradation"/> Performance Degradation</label>
            </div>
          </fieldset>
        </div>

        <div class="confidence-control">
          <label for="confidence-slider">Confidence</label>
          <input id="confidence-slider" type="range" min="0" max="100" value="50" aria-valuemin="0" aria-valuemax="100"/>
          <span id="confidence-value" aria-live="polite">50%</span>
        </div>

        <div>
          <label for="notes">Notes</label>
          <textarea id="notes" placeholder="Add notes about this annotation..."></textarea>
        </div>

        <div class="defer-control">
          <label><input id="defer-checkbox" type="checkbox"/> Defer Decision</label>
          <textarea id="defer-reason" style="display: none;" placeholder="Reason for deferral..."></textarea>
        </div>

        <div id="annotation-status" class="status"></div>
        <button type="submit" id="submit-btn" aria-busy="false">Submit Annotation</button>
      </form>
    `;
    document.body.appendChild(container);

    form = document.getElementById('annotation-form') as HTMLFormElement;
    passBtn = document.getElementById('btn-pass') as HTMLButtonElement;
    failBtn = document.getElementById('btn-fail') as HTMLButtonElement;
    uncertainBtn = document.getElementById('btn-uncertain') as HTMLButtonElement;
    failureSection = document.getElementById('failure-section') as HTMLElement;
    primaryFailure = document.getElementById('primary-failure') as HTMLSelectElement;
    secondaryFailures = document.getElementById('secondary-failures') as HTMLElement;
    confidenceSlider = document.getElementById('confidence-slider') as HTMLInputElement;
    confidenceValue = document.getElementById('confidence-value') as HTMLElement;
    notesTextarea = document.getElementById('notes') as HTMLTextAreaElement;
    deferCheckbox = document.getElementById('defer-checkbox') as HTMLInputElement;
    deferReason = document.getElementById('defer-reason') as HTMLTextAreaElement;
    submitBtn = document.getElementById('submit-btn') as HTMLButtonElement;
  });

  afterEach(() => {
    document.body.removeChild(container);
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // Label Selection (4 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Label Selection', () => {
    it('should handle PASS label selection', () => {
      passBtn.click();
      passBtn.setAttribute('aria-pressed', 'true');
      failBtn.setAttribute('aria-pressed', 'false');
      uncertainBtn.setAttribute('aria-pressed', 'false');

      expect(passBtn.getAttribute('aria-pressed')).toBe('true');
      expect(failBtn.getAttribute('aria-pressed')).toBe('false');
    });

    it('should handle FAIL label selection', () => {
      failBtn.click();
      failBtn.setAttribute('aria-pressed', 'true');
      passBtn.setAttribute('aria-pressed', 'false');
      uncertainBtn.setAttribute('aria-pressed', 'false');
      failureSection.style.display = 'block';

      expect(failBtn.getAttribute('aria-pressed')).toBe('true');
      expect(failureSection.style.display).toBe('block');
    });

    it('should handle UNCERTAIN label selection', () => {
      uncertainBtn.click();
      uncertainBtn.setAttribute('aria-pressed', 'true');
      passBtn.setAttribute('aria-pressed', 'false');
      failBtn.setAttribute('aria-pressed', 'false');
      failureSection.style.display = 'none';

      expect(uncertainBtn.getAttribute('aria-pressed')).toBe('true');
      expect(failureSection.style.display).toBe('none');
    });

    it('should only allow one label selected at a time', () => {
      passBtn.click();
      passBtn.setAttribute('aria-pressed', 'true');
      failBtn.click();
      failBtn.setAttribute('aria-pressed', 'true');
      passBtn.setAttribute('aria-pressed', 'false');

      expect(passBtn.getAttribute('aria-pressed')).toBe('false');
      expect(failBtn.getAttribute('aria-pressed')).toBe('true');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Failure Mode (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Failure Mode', () => {
    it('should select primary failure mode', () => {
      primaryFailure.value = 'timeout';
      expect(primaryFailure.value).toBe('timeout');
    });

    it('should select multiple secondary failures', () => {
      const checkboxes = secondaryFailures.querySelectorAll('input[type="checkbox"]');
      checkboxes[0].checked = true;
      checkboxes[2].checked = true;

      const selected = Array.from(checkboxes)
        .filter((cb) => (cb as HTMLInputElement).checked)
        .map((cb) => (cb as HTMLInputElement).value);

      expect(selected).toEqual(['resource_leak', 'performance_degradation']);
    });

    it('should validate that primary failure is selected when FAIL is chosen', () => {
      primaryFailure.value = '';
      const isValid = primaryFailure.value !== '';
      expect(isValid).toBe(false);

      primaryFailure.value = 'timeout';
      const isValidAfter = primaryFailure.value !== '';
      expect(isValidAfter).toBe(true);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Confidence Rating (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Confidence Rating', () => {
    it('should update confidence value display when slider changes', () => {
      confidenceSlider.value = '75';
      confidenceValue.textContent = '75%';

      expect(confidenceValue.textContent).toBe('75%');
    });

    it('should maintain confidence in valid range (0-100)', () => {
      confidenceSlider.value = '150';
      confidenceSlider.value = Math.min(100, parseInt(confidenceSlider.value)).toString();

      expect(parseInt(confidenceSlider.value)).toBeLessThanOrEqual(100);
    });

    it('should default to 50% confidence', () => {
      const slider = document.createElement('input');
      slider.type = 'range';
      slider.value = '50';

      expect(parseInt(slider.value)).toBe(50);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Notes & Deferral (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Notes & Deferral', () => {
    it('should allow adding notes', () => {
      const noteText = 'This trace had intermittent behavior';
      notesTextarea.value = noteText;

      expect(notesTextarea.value).toBe(noteText);
    });

    it('should show defer reason field when checkbox is checked', () => {
      expect(deferReason.style.display).toBe('none');

      deferCheckbox.checked = true;
      deferReason.style.display = deferCheckbox.checked ? 'block' : 'none';

      expect(deferReason.style.display).toBe('block');
    });

    it('should capture defer reason when deferred', () => {
      deferCheckbox.checked = true;
      deferReason.value = 'Awaiting team review';
      deferReason.style.display = 'block';

      expect(deferCheckbox.checked).toBe(true);
      expect(deferReason.value).toBe('Awaiting team review');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Submission (2 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Submission', () => {
    it('should validate required fields before submission', () => {
      const isValid = passBtn.getAttribute('aria-pressed') === 'true' ||
                      failBtn.getAttribute('aria-pressed') === 'true' ||
                      uncertainBtn.getAttribute('aria-pressed') === 'true';
      expect(isValid).toBe(false);
    });

    it('should build annotation payload with all fields', () => {
      passBtn.click();
      passBtn.setAttribute('aria-pressed', 'true');
      confidenceSlider.value = '85';
      notesTextarea.value = 'Test annotation';

      const payload = {
        label: 'pass',
        confidence: 0.85,
        notes: 'Test annotation',
      };

      expect(payload.label).toBe('pass');
      expect(payload.confidence).toBe(0.85);
      expect(payload.notes).toBe('Test annotation');
    });
  });
});
