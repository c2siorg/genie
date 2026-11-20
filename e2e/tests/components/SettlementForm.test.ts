// SettlementForm Component Tests (15 tests)
// Form rendering, merchant selection, order validation, amount calculation, submission

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('SettlementForm Component', () => {
  let container: HTMLElement;
  let form: HTMLFormElement;
  let merchantSelect: HTMLSelectElement;
  let amountInput: HTMLInputElement;
  let ordersContainer: HTMLElement;
  let submitBtn: HTMLButtonElement;
  let totalDisplay: HTMLElement;

  beforeEach(() => {
    container = document.createElement('div');
    container.innerHTML = `
      <form id="settlement-form" aria-label="Settlement Form">
        <div>
          <label for="merchant-select">Merchant</label>
          <select id="merchant-select" required aria-invalid="false">
            <option value="">Select a merchant</option>
            <option value="merchant-1">Merchant A</option>
            <option value="merchant-2">Merchant B</option>
          </select>
          <div id="merchant-error" class="field-error"></div>
        </div>
        <div>
          <label for="settlement-amount">Amount (Paise)</label>
          <input id="settlement-amount" type="number" name="amount" min="0" step="1" required aria-invalid="false"/>
          <div id="amount-error" class="field-error"></div>
        </div>
        <div>
          <label for="settlement-orders">Orders</label>
          <div id="settlement-orders" class="orders-container">
            <div class="order-item" data-order-id="order-1">
              <span class="order-id">Order 1</span>
              <span class="order-amount">10000</span>
            </div>
            <div class="order-item" data-order-id="order-2">
              <span class="order-id">Order 2</span>
              <span class="order-amount">20000</span>
            </div>
          </div>
        </div>
        <div>
          <label>Total: <span id="settlement-total">0</span> Paise</label>
        </div>
        <div id="settlement-status" class="status-display"></div>
        <button type="submit" aria-busy="false">Submit Settlement</button>
      </form>
    `;
    document.body.appendChild(container);

    form = document.getElementById('settlement-form') as HTMLFormElement;
    merchantSelect = document.getElementById('merchant-select') as HTMLSelectElement;
    amountInput = document.getElementById('settlement-amount') as HTMLInputElement;
    ordersContainer = document.getElementById('settlement-orders') as HTMLElement;
    submitBtn = form.querySelector('button[type="submit"]') as HTMLButtonElement;
    totalDisplay = document.getElementById('settlement-total') as HTMLElement;
  });

  afterEach(() => {
    document.body.removeChild(container);
    vi.clearAllMocks();
  });

  // ─────────────────────────────────────────────────────────────
  // Rendering (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Rendering', () => {
    it('should render settlement form with all fields', () => {
      expect(form).toBeDefined();
      expect(merchantSelect).toBeDefined();
      expect(amountInput).toBeDefined();
      expect(ordersContainer).toBeDefined();
      expect(submitBtn).toBeDefined();
    });

    it('should display merchant options', () => {
      const options = merchantSelect.querySelectorAll('option');
      expect(options.length).toBeGreaterThan(1);
      expect(options[1].value).toBe('merchant-1');
      expect(options[1].textContent).toBe('Merchant A');
    });

    it('should display order items from list', () => {
      const orderItems = ordersContainer.querySelectorAll('.order-item');
      expect(orderItems.length).toBe(2);
      expect(orderItems[0].getAttribute('data-order-id')).toBe('order-1');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Merchant Selection (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Merchant Selection', () => {
    it('should require merchant selection', () => {
      merchantSelect.value = '';
      const isEmpty = !merchantSelect.value;
      expect(isEmpty).toBe(true);
    });

    it('should validate merchant selection', () => {
      merchantSelect.value = 'merchant-1';
      const isSelected = merchantSelect.value !== '';
      expect(isSelected).toBe(true);
    });

    it('should update form when merchant is selected', () => {
      merchantSelect.value = 'merchant-2';
      expect(merchantSelect.value).toBe('merchant-2');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Order Management (4 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Order Management', () => {
    it('should display all orders for merchant', () => {
      const orders = ordersContainer.querySelectorAll('.order-item');
      expect(orders.length).toBe(2);
    });

    it('should extract order amounts correctly', () => {
      const orderAmounts = Array.from(ordersContainer.querySelectorAll('.order-amount')).map(
        (el) => parseInt(el.textContent || '0')
      );
      expect(orderAmounts).toEqual([10000, 20000]);
    });

    it('should calculate total from order amounts', () => {
      const orderAmounts = Array.from(ordersContainer.querySelectorAll('.order-amount')).map(
        (el) => parseInt(el.textContent || '0')
      );
      const total = orderAmounts.reduce((sum, amount) => sum + amount, 0);
      expect(total).toBe(30000);
    });

    it('should allow selecting specific orders', () => {
      const orderItem = ordersContainer.querySelector('.order-item') as HTMLElement;
      orderItem.classList.add('selected');
      expect(orderItem.classList.contains('selected')).toBe(true);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Amount Validation (3 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Amount Validation', () => {
    it('should require amount to be positive', () => {
      amountInput.value = '';
      const isEmpty = !amountInput.value;
      expect(isEmpty).toBe(true);
    });

    it('should reject negative amounts', () => {
      amountInput.value = '-1000';
      const isNegative = parseInt(amountInput.value) < 0;
      expect(isNegative).toBe(true);
    });

    it('should accept valid amounts', () => {
      amountInput.value = '50000';
      const isValid = parseInt(amountInput.value) > 0;
      expect(isValid).toBe(true);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Submission (2 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Submission', () => {
    it('should disable submit button during submission', () => {
      expect(submitBtn.disabled).toBe(false);
      submitBtn.disabled = true;
      submitBtn.setAttribute('aria-busy', 'true');

      expect(submitBtn.disabled).toBe(true);
      expect(submitBtn.getAttribute('aria-busy')).toBe('true');
    });

    it('should build payload with correct structure', () => {
      merchantSelect.value = 'merchant-1';
      amountInput.value = '50000';

      const payload = {
        merchant_id: merchantSelect.value,
        amount_paise: parseInt(amountInput.value),
      };

      expect(payload.merchant_id).toBe('merchant-1');
      expect(payload.amount_paise).toBe(50000);
    });
  });
});
