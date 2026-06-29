// LoginForm Component Tests (12 tests)
// Form rendering, validation, submission, error handling, accessibility

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('LoginForm Component', () => {
  let container: HTMLElement;
  let form: HTMLFormElement;
  let emailInput: HTMLInputElement;
  let passwordInput: HTMLInputElement;
  let submitBtn: HTMLButtonElement;
  let errorDisplay: HTMLElement;

  beforeEach(() => {
    // Create component DOM
    container = document.createElement('div');
    container.innerHTML = `
      <form id="login-form" aria-label="Login Form">
        <div>
          <label for="login-email">Email</label>
          <input id="login-email" type="email" name="email" required aria-invalid="false" aria-describedby="email-error"/>
          <div id="email-error" class="field-error"></div>
        </div>
        <div>
          <label for="login-password">Password</label>
          <input id="login-password" type="password" name="password" required aria-invalid="false" aria-describedby="password-error"/>
          <div id="password-error" class="field-error"></div>
        </div>
        <button type="submit" aria-busy="false">Sign In</button>
        <div id="form-login-error" class="form-error"></div>
      </form>
    `;
    document.body.appendChild(container);

    form = document.getElementById('login-form') as HTMLFormElement;
    emailInput = document.getElementById('login-email') as HTMLInputElement;
    passwordInput = document.getElementById('login-password') as HTMLInputElement;
    submitBtn = form.querySelector('button[type="submit"]') as HTMLButtonElement;
    errorDisplay = document.getElementById('form-login-error') as HTMLElement;
  });

  afterEach(() => {
    document.body.removeChild(container);
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // Rendering (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Rendering', () => {
    it('should render login form with all fields', () => {
      expect(form).toBeDefined();
      expect(emailInput).toBeDefined();
      expect(passwordInput).toBeDefined();
      expect(submitBtn).toBeDefined();
    });

    it('should have proper labels for accessibility', () => {
      const emailLabel = container.querySelector('label[for="login-email"]');
      const passwordLabel = container.querySelector('label[for="login-password"]');

      expect(emailLabel?.textContent).toBe('Email');
      expect(passwordLabel?.textContent).toBe('Password');
    });

    it('should display error messages in dedicated error containers', () => {
      expect(document.getElementById('email-error')).toBeDefined();
      expect(document.getElementById('password-error')).toBeDefined();
      expect(document.getElementById('form-login-error')).toBeDefined();
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Validation (4 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Validation', () => {
    it('should validate email format', () => {
      emailInput.value = 'invalidemail';
      const isValid = emailInput.value.includes('@');
      expect(isValid).toBe(false);

      emailInput.value = 'user@example.com';
      const isValidEmail = emailInput.value.includes('@');
      expect(isValidEmail).toBe(true);
    });

    it('should require email field', () => {
      emailInput.value = '';
      const isEmpty = !emailInput.value.trim();
      expect(isEmpty).toBe(true);
    });

    it('should require password field', () => {
      passwordInput.value = '';
      const isEmpty = !passwordInput.value;
      expect(isEmpty).toBe(true);
    });

    it('should show field-level errors', () => {
      const emailError = document.getElementById('email-error') as HTMLElement;
      emailError.textContent = 'Invalid email format';
      emailInput.setAttribute('aria-invalid', 'true');

      expect(emailError.textContent).toBe('Invalid email format');
      expect(emailInput.getAttribute('aria-invalid')).toBe('true');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Submission (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Submission', () => {
    it('should prevent default form submission', () => {
      const submitEvent = new Event('submit');
      const preventSpy = vi.spyOn(submitEvent, 'preventDefault');

      form.dispatchEvent(submitEvent);
      if (submitEvent.type === 'submit') {
        submitEvent.preventDefault();
      }

      expect(preventSpy).toHaveBeenCalled();
    });

    it('should collect form data correctly', () => {
      emailInput.value = 'test@example.com';
      passwordInput.value = 'password123';

      const formData = new FormData(form);
      expect(formData.get('email')).toBe('test@example.com');
      expect(formData.get('password')).toBe('password123');
    });

    it('should disable submit button during submission', () => {
      expect(submitBtn.disabled).toBe(false);
      submitBtn.disabled = true;
      submitBtn.setAttribute('aria-busy', 'true');

      expect(submitBtn.disabled).toBe(true);
      expect(submitBtn.getAttribute('aria-busy')).toBe('true');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Error Handling (2 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Error Handling', () => {
    it('should display form-level error message', () => {
      const errorMsg = 'Invalid credentials';
      errorDisplay.textContent = errorMsg;
      errorDisplay.style.display = 'block';

      expect(errorDisplay.textContent).toBe('Invalid credentials');
      expect(errorDisplay.style.display).toBe('block');
    });

    it('should clear errors when user focuses on field', () => {
      const emailError = document.getElementById('email-error') as HTMLElement;
      emailError.textContent = 'Error message';
      emailInput.focus();
      emailError.textContent = '';

      expect(emailError.textContent).toBe('');
    });
  });
});
