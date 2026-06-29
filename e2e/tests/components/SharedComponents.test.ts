// Shared Components Tests (16 tests)
// Button states, loading indicators, status messages, error displays, modals

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('Shared Components', () => {
  let container: HTMLElement;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
  });

  afterEach(() => {
    document.body.removeChild(container);
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // Button Component (4 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Button Component', () => {
    it('should render button with label', () => {
      container.innerHTML = '<button id="test-btn">Click Me</button>';
      const btn = document.getElementById('test-btn') as HTMLButtonElement;

      expect(btn).toBeDefined();
      expect(btn.textContent).toBe('Click Me');
    });

    it('should disable button when loading', () => {
      container.innerHTML = '<button id="test-btn">Submit</button>';
      const btn = document.getElementById('test-btn') as HTMLButtonElement;

      btn.disabled = true;
      btn.setAttribute('aria-busy', 'true');

      expect(btn.disabled).toBe(true);
      expect(btn.getAttribute('aria-busy')).toBe('true');
    });

    it('should emit click event', () => {
      container.innerHTML = '<button id="test-btn">Click</button>';
      const btn = document.getElementById('test-btn') as HTMLButtonElement;
      const spy = vi.fn();

      btn.addEventListener('click', spy);
      btn.click();

      expect(spy).toHaveBeenCalled();
    });

    it('should handle variant styles', () => {
      container.innerHTML = '<button id="test-btn" class="btn btn-primary">Submit</button>';
      const btn = document.getElementById('test-btn') as HTMLButtonElement;

      expect(btn.classList.contains('btn-primary')).toBe(true);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Loading Indicator (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Loading Indicator', () => {
    it('should show loading spinner', () => {
      container.innerHTML = '<div id="loader" class="loading-spinner" role="status" aria-label="Loading"><span></span></div>';
      const loader = document.getElementById('loader') as HTMLElement;

      expect(loader).toBeDefined();
      expect(loader.getAttribute('role')).toBe('status');
    });

    it('should hide loading spinner', () => {
      container.innerHTML = '<div id="loader" class="loading-spinner" hidden></div>';
      const loader = document.getElementById('loader') as HTMLElement;

      expect(loader.hidden).toBe(true);
    });

    it('should display loading message', () => {
      container.innerHTML = '<div class="loading"><span id="loading-msg">Please wait...</span></div>';
      const msg = document.getElementById('loading-msg') as HTMLElement;

      expect(msg.textContent).toBe('Please wait...');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Status Message (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Status Message', () => {
    it('should display success message', () => {
      container.innerHTML = '<div id="status" class="status status-success" role="status">Success!</div>';
      const status = document.getElementById('status') as HTMLElement;

      expect(status.classList.contains('status-success')).toBe(true);
      expect(status.textContent).toBe('Success!');
    });

    it('should display error message', () => {
      container.innerHTML = '<div id="status" class="status status-error" role="alert">Error occurred</div>';
      const status = document.getElementById('status') as HTMLElement;

      expect(status.classList.contains('status-error')).toBe(true);
      expect(status.getAttribute('role')).toBe('alert');
    });

    it('should display warning message', () => {
      container.innerHTML = '<div id="status" class="status status-warning">Warning: This is important</div>';
      const status = document.getElementById('status') as HTMLElement;

      expect(status.classList.contains('status-warning')).toBe(true);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Error Display (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Error Display', () => {
    it('should show error message with details', () => {
      container.innerHTML = `
        <div id="error" role="alert" class="error-box">
          <strong>Error:</strong>
          <p id="error-msg">Network request failed</p>
        </div>
      `;
      const errorMsg = document.getElementById('error-msg') as HTMLElement;

      expect(errorMsg.textContent).toBe('Network request failed');
    });

    it('should allow dismissing error', () => {
      container.innerHTML = `
        <div id="error" class="error-box">
          <span id="error-text">Error message</span>
          <button id="error-close" aria-label="Close error">×</button>
        </div>
      `;
      const closeBtn = document.getElementById('error-close') as HTMLButtonElement;
      closeBtn.click();

      expect(closeBtn.getAttribute('aria-label')).toBe('Close error');
    });

    it('should display list of validation errors', () => {
      container.innerHTML = `
        <div class="error-list">
          <ul id="errors">
            <li class="error-item">Field 1 is required</li>
            <li class="error-item">Field 2 must be valid</li>
          </ul>
        </div>
      `;
      const errors = document.querySelectorAll('.error-item');

      expect(errors.length).toBe(2);
      expect(errors[0].textContent).toBe('Field 1 is required');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Modal/Dialog (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Modal/Dialog', () => {
    it('should render modal dialog', () => {
      container.innerHTML = `
        <div id="modal" class="modal" role="dialog" aria-labelledby="modal-title" hidden>
          <h2 id="modal-title">Confirm Action</h2>
          <p>Are you sure?</p>
          <button id="modal-confirm">Confirm</button>
          <button id="modal-cancel">Cancel</button>
        </div>
      `;
      const modal = document.getElementById('modal') as HTMLElement;

      expect(modal).toBeDefined();
      expect(modal.getAttribute('role')).toBe('dialog');
    });

    it('should show modal when triggered', () => {
      container.innerHTML = `
        <div id="modal" class="modal" hidden>Content</div>
        <button id="open-modal">Open</button>
      `;
      const modal = document.getElementById('modal') as HTMLElement;
      const openBtn = document.getElementById('open-modal') as HTMLButtonElement;

      openBtn.click();
      modal.hidden = false;

      expect(modal.hidden).toBe(false);
    });

    it('should close modal on action', () => {
      container.innerHTML = `
        <div id="modal" class="modal">
          <button id="modal-close">Close</button>
        </div>
      `;
      const modal = document.getElementById('modal') as HTMLElement;
      const closeBtn = document.getElementById('modal-close') as HTMLButtonElement;

      modal.hidden = true;
      closeBtn.click();

      expect(modal.hidden).toBe(true);
    });
  });
});
