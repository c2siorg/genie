import { Page, expect } from '@playwright/test';

/**
 * Settlement Page Object
 * Encapsulates settlement workflow UI interactions
 */
export class SettlementPage {
  constructor(private page: Page) {}

  // Selectors
  private readonly ORDER_ID_INPUT = '[data-testid="order-id-input"]';
  private readonly AMOUNT_INPUT = '[data-testid="amount-input"]';
  private readonly INITIATE_PAYMENT_BTN = '[data-testid="initiate-payment-btn"]';
  private readonly CONFIRM_PAYMENT_BTN = '[data-testid="confirm-payment-btn"]';
  private readonly SETTLEMENT_STATUS = '[data-testid="settlement-status"]';
  private readonly RECONCILIATION_CHECK = '[data-testid="reconciliation-check"]';
  private readonly CBDC_COMMIT_STATUS = '[data-testid="cbdc-commit-status"]';
  private readonly NETTING_AMOUNT = '[data-testid="netting-amount"]';
  private readonly ERROR_MESSAGE = '[data-testid="error-message"]';
  private readonly SUCCESS_MESSAGE = '[data-testid="success-message"]';
  private readonly SETTLEMENT_TABLE = '[data-testid="settlement-table"]';
  private readonly RETRY_BTN = '[data-testid="retry-btn"]';

  /**
   * Navigate to settlement page
   */
  async goto() {
    await this.page.goto('/settlement');
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Initiate payment workflow
   */
  async initiatePayment(orderId: string, amount: string) {
    await this.page.fill(this.ORDER_ID_INPUT, orderId);
    await this.page.fill(this.AMOUNT_INPUT, amount);
    await this.page.click(this.INITIATE_PAYMENT_BTN);
    await this.page.waitForSelector(this.CONFIRM_PAYMENT_BTN);
  }

  /**
   * Confirm payment
   */
  async confirmPayment() {
    await this.page.click(this.CONFIRM_PAYMENT_BTN);
    await this.waitForSettlementCompletion();
  }

  /**
   * Wait for settlement to complete
   */
  async waitForSettlementCompletion(timeout = 30000) {
    await this.page.waitForSelector('[data-status="COMPLETED"]', { timeout });
  }

  /**
   * Get settlement status
   */
  async getSettlementStatus(): Promise<string> {
    const status = await this.page.textContent(this.SETTLEMENT_STATUS);
    return status?.trim() || '';
  }

  /**
   * Verify CBDC commitment
   */
  async verifyCBDCCommit(): Promise<boolean> {
    const cbdcStatus = await this.page.textContent(this.CBDC_COMMIT_STATUS);
    return cbdcStatus?.includes('COMMITTED') || false;
  }

  /**
   * Verify reconciliation passed
   */
  async verifyReconciliation(): Promise<boolean> {
    const reconcStatus = await this.page.textContent(this.RECONCILIATION_CHECK);
    return reconcStatus?.includes('VERIFIED') || false;
  }

  /**
   * Get netting amount
   */
  async getNettingAmount(): Promise<string> {
    const netting = await this.page.textContent(this.NETTING_AMOUNT);
    return netting?.trim() || '0';
  }

  /**
   * Check for error message
   */
  async getErrorMessage(): Promise<string | null> {
    const error = await this.page.textContent(this.ERROR_MESSAGE);
    return error?.trim() || null;
  }

  /**
   * Check for success message
   */
  async getSuccessMessage(): Promise<string | null> {
    const success = await this.page.textContent(this.SUCCESS_MESSAGE);
    return success?.trim() || null;
  }

  /**
   * Retry failed settlement
   */
  async retrySettlement() {
    await this.page.click(this.RETRY_BTN);
    await this.waitForSettlementCompletion();
  }

  /**
   * Get all settlements from table
   */
  async getSettlementsFromTable() {
    const rows = await this.page.locator(`${this.SETTLEMENT_TABLE} tbody tr`).count();
    const settlements = [];

    for (let i = 0; i < rows; i++) {
      const row = this.page.locator(`${this.SETTLEMENT_TABLE} tbody tr`).nth(i);
      const orderId = await row.locator('td:nth-child(1)').textContent();
      const status = await row.locator('td:nth-child(2)').textContent();
      const amount = await row.locator('td:nth-child(3)').textContent();

      settlements.push({
        orderId: orderId?.trim(),
        status: status?.trim(),
        amount: amount?.trim(),
      });
    }

    return settlements;
  }

  /**
   * Verify settlement amount correctness
   */
  async verifyAmountCorrectness(expectedAmount: string): Promise<boolean> {
    const actualAmount = await this.page.locator('[data-testid="settlement-amount"]').textContent();
    return actualAmount?.includes(expectedAmount) || false;
  }

  /**
   * Get audit trail entries
   */
  async getAuditTrail() {
    const entries = await this.page.locator('[data-testid="audit-entry"]').count();
    const trail = [];

    for (let i = 0; i < entries; i++) {
      const entry = this.page.locator('[data-testid="audit-entry"]').nth(i);
      const step = await entry.locator('[data-testid="step-name"]').textContent();
      const timestamp = await entry.locator('[data-testid="timestamp"]').textContent();

      trail.push({
        step: step?.trim(),
        timestamp: timestamp?.trim(),
      });
    }

    return trail;
  }
}
