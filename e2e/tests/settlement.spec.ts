import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';
import { apiFixture } from '../fixtures/api.fixture';
import { SettlementPage } from '../pages/SettlementPage';

/**
 * Settlement Workflow E2E Tests
 *
 * Coverage:
 * - Order creation → Payment → Settlement → Fulfillment
 * - Amount correctness verification
 * - CBDC ledger integration
 * - Netting calculations
 * - Reconciliation verification
 * - Audit trail integrity
 */

const testFixtures = authFixture.extend(apiFixture);

test.describe('Settlement Workflow', () => {
  let settlementPage: SettlementPage;

  test.beforeEach(async ({ page }) => {
    settlementPage = new SettlementPage(page);
    await settlementPage.goto();
  });

  test.describe('Happy Path: Single Order Settlement', () => {
    test('should create order and complete settlement successfully', async ({
      page,
      api,
      getTestUserToken,
    }) => {
      const userToken = getTestUserToken();

      // Create order via API
      const order = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 100000, // ₹1000
          items: [
            {
              name: 'Item 1',
              quantity: 1,
              price_paise: 100000,
            },
          ],
        },
        userToken
      );

      expect(order.order_id).toBeDefined();
      expect(order.status).toBe('CREATED');

      const orderId = order.order_id;

      // Initiate payment
      const payment = await api.initiatePayment(
        {
          order_id: orderId,
          amount_paise: 100000,
          payment_method: 'erupeepayment',
        },
        userToken
      );

      expect(payment.payment_id).toBeDefined();
      expect(payment.status).toBe('INITIATED');

      // Confirm payment
      const confirmed = await api.confirmPayment(payment.payment_id, userToken);
      expect(confirmed.status).toBe('CONFIRMED');

      // Verify settlement
      const settlement = await api.getSettlement(confirmed.settlement_id, userToken);
      expect(settlement.status).toBe('COMPLETED');
      expect(settlement.amount_paise).toBe(100000);
      expect(settlement.cbdc_committed).toBe(true);
      expect(settlement.reconciliation_passed).toBe(true);
    });

    test('should verify settlement amount correctness', async ({ page, api, getTestUserToken }) => {
      const userToken = getTestUserToken();
      const expectedAmount = '1000'; // ₹1000

      // Create and settle order
      const order = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
        },
        userToken
      );

      await api.confirmPayment(payment.payment_id, userToken);

      // Verify UI shows correct amount
      const amountCorrect = await settlementPage.verifyAmountCorrectness(expectedAmount);
      expect(amountCorrect).toBe(true);
    });

    test('should verify CBDC ledger commitment', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      const order = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
        },
        userToken
      );

      await api.confirmPayment(payment.payment_id, userToken);

      // Verify CBDC commitment
      const cbdcCommitted = await settlementPage.verifyCBDCCommit();
      expect(cbdcCommitted).toBe(true);
    });

    test('should verify reconciliation check passes', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      const order = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
        },
        userToken
      );

      await api.confirmPayment(payment.payment_id, userToken);

      // Verify reconciliation passed
      const reconciliationPassed = await settlementPage.verifyReconciliation();
      expect(reconciliationPassed).toBe(true);
    });
  });

  test.describe('Multiple Order Settlement with Netting', () => {
    test('should settle multiple orders with netting', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      // Create multiple orders from same merchant
      const order1 = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const order2 = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-002',
          total_paise: 50000,
          items: [],
        },
        userToken
      );

      // Initiate and confirm both payments
      const payment1 = await api.initiatePayment(
        {
          order_id: order1.order_id,
          amount_paise: 100000,
        },
        userToken
      );
      await api.confirmPayment(payment1.payment_id, userToken);

      const payment2 = await api.initiatePayment(
        {
          order_id: order2.order_id,
          amount_paise: 50000,
        },
        userToken
      );
      await api.confirmPayment(payment2.payment_id, userToken);

      // Verify netting was applied
      const nettingAmount = await settlementPage.getNettingAmount();
      expect(parseFloat(nettingAmount)).toBeGreaterThan(0);
    });
  });

  test.describe('Error Handling', () => {
    test('should handle insufficient balance error', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      const order = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-invalid',
          total_paise: 999999999, // Unrealistic amount
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 999999999,
        },
        userToken
      );

      // Confirm should fail with insufficient balance
      try {
        await api.confirmPayment(payment.payment_id, userToken);
      } catch (error) {
        expect(error).toBeDefined();
      }

      // Check error message displayed
      const errorMsg = await settlementPage.getErrorMessage();
      expect(errorMsg).toContain('insufficient');
    });

    test('should handle reconciliation failure', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      // Create order that will fail reconciliation
      const order = await api.createOrder(
        {
          merchant_id: 'merchant-bad',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
        },
        userToken
      );

      await api.confirmPayment(payment.payment_id, userToken);

      // Verify reconciliation fails
      const reconciliationPassed = await settlementPage.verifyReconciliation();
      expect(reconciliationPassed).toBe(false);
    });

    test('should allow retry on failed settlement', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      const order = await api.createOrder(
        {
          merchant_id: 'merchant-retry',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
        },
        userToken
      );

      await api.confirmPayment(payment.payment_id, userToken);

      // Retry settlement
      await settlementPage.retrySettlement();

      // Should eventually succeed or show clear error
      const status = await settlementPage.getSettlementStatus();
      expect(status).toMatch(/COMPLETED|FAILED|RETRYING/);
    });
  });

  test.describe('Audit Trail Verification', () => {
    test('should record complete audit trail', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      const order = await api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 100000,
          items: [],
        },
        userToken
      );

      const payment = await api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
        },
        userToken
      );

      await api.confirmPayment(payment.payment_id, userToken);

      // Get audit trail
      const auditTrail = await settlementPage.getAuditTrail();

      // Verify all expected steps are logged
      const steps = auditTrail.map((entry) => entry.step);
      expect(steps).toContain('order_created');
      expect(steps).toContain('payment_initiated');
      expect(steps).toContain('payment_confirmed');
      expect(steps).toContain('settlement_completed');

      // Verify timestamps are monotonic
      let lastTime = 0;
      for (const entry of auditTrail) {
        const timestamp = new Date(entry.timestamp || 0).getTime();
        expect(timestamp).toBeGreaterThanOrEqual(lastTime);
        lastTime = timestamp;
      }
    });
  });

  test.describe('Settlement Table View', () => {
    test('should display all settlements in table', async ({ api, getTestUserToken }) => {
      const userToken = getTestUserToken();

      // Create 3 orders
      for (let i = 0; i < 3; i++) {
        const order = await api.createOrder(
          {
            merchant_id: 'merchant-001',
            customer_id: `customer-${i}`,
            total_paise: 100000,
            items: [],
          },
          userToken
        );

        const payment = await api.initiatePayment(
          {
            order_id: order.order_id,
            amount_paise: 100000,
          },
          userToken
        );

        await api.confirmPayment(payment.payment_id, userToken);
      }

      // Get settlements from table
      const settlements = await settlementPage.getSettlementsFromTable();

      expect(settlements.length).toBe(3);
      settlements.forEach((settlement) => {
        expect(settlement.orderId).toBeDefined();
        expect(settlement.status).toBe('COMPLETED');
        expect(settlement.amount).toBeDefined();
      });
    });
  });
});
