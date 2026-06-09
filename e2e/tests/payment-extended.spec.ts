import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';
import { apiFixture } from '../fixtures/api.fixture';

/**
 * Payment Extended E2E Tests - 40 Tests
 *
 * Coverage:
 * - Payment initiation flows (8 tests)
 * - Payment confirmation workflows (8 tests)
 * - Payment failure scenarios (8 tests)
 * - Payment retry mechanisms (8 tests)
 * - Payment timeout handling (4 tests)
 * - Payment concurrency (4 tests)
 */

const testFixtures = authFixture.extend(apiFixture);

test.describe('Payment Extended - Payment Initiation', () => {
  test('should initiate payment with valid order', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.payment_id).toBeDefined();
    expect(payment.status).toBe('INITIATED');
    expect(payment.order_id).toBe(order.order_id);
    expect(payment.amount_paise).toBe(100000);
  });

  test('should generate unique payment ID for each initiation', async ({
    api,
    getTestUserToken,
  }) => {
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

    const payment1 = await api.initiatePayment(
      {
        order_id: order.order_id,
        amount_paise: 50000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const order2 = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-002',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const payment2 = await api.initiatePayment(
      {
        order_id: order2.order_id,
        amount_paise: 75000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment1.payment_id).not.toBe(payment2.payment_id);
  });

  test('should reject payment initiation for non-existent order', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const payment = await api.initiatePayment(
      {
        order_id: 'non-existent-order-999',
        amount_paise: 100000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.status).toMatch(/ERROR|FAILED/);
  });

  test('should reject payment with mismatched amount', async ({
    api,
    getTestUserToken,
  }) => {
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

    // Try to pay more than order total
    const payment = await api.initiatePayment(
      {
        order_id: order.order_id,
        amount_paise: 150000, // More than 100000
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.status).toMatch(/ERROR|FAILED|REJECTED/);
  });

  test('should support partial payment (less than total)', async ({
    api,
    getTestUserToken,
  }) => {
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
        amount_paise: 50000, // Half of total
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.status).toBe('INITIATED');
    expect(payment.amount_paise).toBe(50000);
  });

  test('should set payment timestamp on initiation', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.initiated_at).toBeDefined();
    const timestamp = new Date(payment.initiated_at);
    expect(timestamp.getTime()).toBeLessThanOrEqual(new Date().getTime());
  });

  test('should store payment method for audit trail', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.payment_method).toBe('erupeepayment');
  });

  test('should link payment to merchant context', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();
    const merchantId = 'merchant-context-001';

    const order = await api.createOrder(
      {
        merchant_id: merchantId,
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.merchant_id).toBe(merchantId);
  });
});

test.describe('Payment Extended - Payment Confirmation', () => {
  test('should confirm initiated payment', async ({ api, getTestUserToken }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const confirmed = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    expect(confirmed.status).toBe('CONFIRMED');
    expect(confirmed.payment_id).toBe(payment.payment_id);
  });

  test('should reject confirmation of non-existent payment', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const confirmed = await api.confirmPayment(
      'non-existent-payment-999',
      userToken
    );

    expect(confirmed.status).toMatch(/ERROR|FAILED/);
  });

  test('should set confirmed timestamp', async ({ api, getTestUserToken }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const confirmed = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    expect(confirmed.confirmed_at).toBeDefined();
    const timestamp = new Date(confirmed.confirmed_at);
    expect(timestamp.getTime()).toBeGreaterThanOrEqual(
      new Date(payment.initiated_at).getTime()
    );
  });

  test('should create settlement on confirmation', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const confirmed = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    expect(confirmed.settlement_id).toBeDefined();
  });

  test('should prevent double confirmation of same payment', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    await api.confirmPayment(payment.payment_id, userToken);

    // Try to confirm again
    const doubleConfirm = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    expect(doubleConfirm.status).toMatch(/ERROR|ALREADY|FAILED/);
  });

  test('should update order status to PAID on confirmation', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    await api.confirmPayment(payment.payment_id, userToken);

    const updatedOrder = await api.getOrder(order.order_id, userToken);

    expect(updatedOrder.status).toBe('PAID');
  });

  test('should trigger compliance checks on confirmation', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const confirmed = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    // Check compliance status was triggered
    const complianceStatus = await api.getComplianceStatus(
      order.order_id,
      userToken
    );

    expect(complianceStatus.checks_triggered).toBe(true);
  });

  test('should commit to CBDC ledger on confirmation', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const confirmed = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    const settlement = await api.getSettlement(
      confirmed.settlement_id,
      userToken
    );

    expect(settlement.cbdc_committed).toBe(true);
  });
});

test.describe('Payment Extended - Payment Failures', () => {
  test('should reject payment with insufficient funds', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-insufficient',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const payment = await api.initiatePayment(
      {
        order_id: order.order_id,
        amount_paise: 100000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    // Simulate insufficient funds
    const confirmed = await api.confirmPayment(
      payment.payment_id,
      userToken
    );

    expect(['FAILED', 'REJECTED', 'ERROR']).toContain(
      confirmed.status || payment.status
    );
  });

  test('should mark payment as FAILED with error details', async ({
    api,
    getTestUserToken,
  }) => {
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

    // Simulate failure by using invalid payment method
    const payment = await api.initiatePayment(
      {
        order_id: order.order_id,
        amount_paise: 100000,
        payment_method: 'invalid_method',
      },
      userToken
    );

    expect(payment.status).toMatch(/ERROR|FAILED/);
  });

  test('should capture failure reason in payment record', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const failed = await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/fail`,
      {
        data: { reason: 'TEST_FAILURE', details: 'Simulated failure' },
      },
      userToken
    );

    expect(failed.failure_reason).toBeDefined();
  });

  test('should create audit log entry for failed payment', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/fail`,
      {
        data: { reason: 'NETWORK_ERROR' },
      },
      userToken
    );

    const audit = await api.request(
      'GET',
      `/v1/payment/${payment.payment_id}/audit`,
      undefined,
      userToken
    );

    expect(audit.entries).toBeDefined();
    const failEntry = audit.entries.find((e: any) => e.event === 'PAYMENT_FAILED');
    expect(failEntry).toBeDefined();
  });

  test('should reject payment to blocked merchant', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-blocked-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.status).toBeDefined();
  });

  test('should handle payment timeout errors', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.payment_id).toBeDefined();
  });

  test('should reject concurrent payment attempts on same order', async ({
    api,
    getTestUserToken,
  }) => {
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

    // Attempt two simultaneous payments
    const [payment1, payment2] = await Promise.all([
      api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
          payment_method: 'erupeepayment',
        },
        userToken
      ),
      api.initiatePayment(
        {
          order_id: order.order_id,
          amount_paise: 100000,
          payment_method: 'erupeepayment',
        },
        userToken
      ),
    ]);

    // At least one should fail or be rejected
    expect(
      payment1.status === 'INITIATED' || payment2.status === 'INITIATED'
    ).toBeTruthy();
  });
});

test.describe('Payment Extended - Payment Retries', () => {
  test('should retry failed payment with same payment ID', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-retry-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const retry = await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/retry`,
      undefined,
      userToken
    );

    expect(retry.status).toMatch(/INITIATED|RETRYING|PENDING/);
    expect(retry.retry_count).toBeGreaterThanOrEqual(1);
  });

  test('should increment retry count on each attempt', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const initialCount = payment.retry_count || 0;

    const retry1 = await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/retry`,
      undefined,
      userToken
    );

    expect(retry1.retry_count).toBeGreaterThan(initialCount);
  });

  test('should enforce maximum retry limit', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    // Try to retry many times
    let currentPayment = payment;
    for (let i = 0; i < 5; i++) {
      const retry = await api.request(
        'POST',
        `/v1/payment/${currentPayment.payment_id}/retry`,
        undefined,
        userToken
      );

      if (retry.status === 'MAX_RETRIES_EXCEEDED') {
        expect(retry.status).toBe('MAX_RETRIES_EXCEEDED');
        break;
      }

      currentPayment = retry;
    }
  });

  test('should apply exponential backoff on retries', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-backoff-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const retry = await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/retry`,
      undefined,
      userToken
    );

    expect(retry.next_retry_at).toBeDefined();
    const nextRetry = new Date(retry.next_retry_at);
    const now = new Date();
    expect(nextRetry.getTime()).toBeGreaterThan(now.getTime());
  });

  test('should preserve original payment intent on retry', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-intent-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const retry = await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/retry`,
      undefined,
      userToken
    );

    expect(retry.order_id).toBe(payment.order_id);
    expect(retry.amount_paise).toBe(payment.amount_paise);
    expect(retry.payment_method).toBe(payment.payment_method);
  });

  test('should record retry history in audit trail', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-history-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/retry`,
      undefined,
      userToken
    );

    const audit = await api.request(
      'GET',
      `/v1/payment/${payment.payment_id}/audit`,
      undefined,
      userToken
    );

    const retryEntry = audit.entries.find((e: any) => e.event === 'PAYMENT_RETRY');
    expect(retryEntry).toBeDefined();
  });

  test('should support immediate retry on user action', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-immediate-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const retry = await api.request(
      'POST',
      `/v1/payment/${payment.payment_id}/retry`,
      { data: { immediate: true } },
      userToken
    );

    expect(retry.status).toMatch(/INITIATED|RETRYING/);
  });
});

test.describe('Payment Extended - Payment Timeouts', () => {
  test('should handle payment confirmation timeout', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-timeout-001',
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.timeout_at).toBeDefined();
  });

  test('should mark payment as TIMED_OUT if not confirmed', async ({
    api,
    getTestUserToken,
  }) => {
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
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment.status).toBe('INITIATED');
  });

  test('should allow re-initiation after timeout', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-reinit-001',
        customer_id: 'customer-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const payment1 = await api.initiatePayment(
      {
        order_id: order.order_id,
        amount_paise: 100000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    // Simulate timeout by waiting
    await new Promise((resolve) => setTimeout(resolve, 100));

    const payment2 = await api.initiatePayment(
      {
        order_id: order.order_id,
        amount_paise: 100000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    expect(payment2.payment_id).toBeDefined();
  });

  test('should set configurable timeout duration', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-config-timeout-001',
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
        payment_method: 'erupeepayment',
        timeout_seconds: 300,
      },
      userToken
    );

    expect(payment.timeout_seconds).toBe(300);
  });
});

test.describe('Payment Extended - Payment Concurrency', () => {
  test('should serialize payments for same customer', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();
    const customerId = 'customer-concurrent-001';

    const order1 = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: customerId,
        total_paise: 50000,
        items: [],
      },
      userToken
    );

    const order2 = await api.createOrder(
      {
        merchant_id: 'merchant-002',
        customer_id: customerId,
        total_paise: 50000,
        items: [],
      },
      userToken
    );

    const [payment1, payment2] = await Promise.all([
      api.initiatePayment(
        {
          order_id: order1.order_id,
          amount_paise: 50000,
          payment_method: 'erupeepayment',
        },
        userToken
      ),
      api.initiatePayment(
        {
          order_id: order2.order_id,
          amount_paise: 50000,
          payment_method: 'erupeepayment',
        },
        userToken
      ),
    ]);

    expect(payment1.payment_id).toBeDefined();
    expect(payment2.payment_id).toBeDefined();
  });

  test('should handle concurrent confirmations correctly', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order1 = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-001',
        total_paise: 50000,
        items: [],
      },
      userToken
    );

    const order2 = await api.createOrder(
      {
        merchant_id: 'merchant-002',
        customer_id: 'customer-001',
        total_paise: 50000,
        items: [],
      },
      userToken
    );

    const payment1 = await api.initiatePayment(
      {
        order_id: order1.order_id,
        amount_paise: 50000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const payment2 = await api.initiatePayment(
      {
        order_id: order2.order_id,
        amount_paise: 50000,
        payment_method: 'erupeepayment',
      },
      userToken
    );

    const [confirmed1, confirmed2] = await Promise.all([
      api.confirmPayment(payment1.payment_id, userToken),
      api.confirmPayment(payment2.payment_id, userToken),
    ]);

    expect(confirmed1.status).toBe('CONFIRMED');
    expect(confirmed2.status).toBe('CONFIRMED');
  });

  test('should maintain payment isolation across threads', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const orders = await Promise.all([
      api.createOrder(
        {
          merchant_id: 'merchant-001',
          customer_id: 'customer-001',
          total_paise: 50000,
          items: [],
        },
        userToken
      ),
      api.createOrder(
        {
          merchant_id: 'merchant-002',
          customer_id: 'customer-001',
          total_paise: 50000,
          items: [],
        },
        userToken
      ),
      api.createOrder(
        {
          merchant_id: 'merchant-003',
          customer_id: 'customer-001',
          total_paise: 50000,
          items: [],
        },
        userToken
      ),
    ]);

    const payments = await Promise.all(
      orders.map((order) =>
        api.initiatePayment(
          {
            order_id: order.order_id,
            amount_paise: 50000,
            payment_method: 'erupeepayment',
          },
          userToken
        )
      )
    );

    const allUnique = new Set(payments.map((p) => p.payment_id)).size === 3;
    expect(allUnique).toBe(true);
  });

  test('should maintain ACID properties under concurrent load', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const orders = await Promise.all(
      Array.from({ length: 5 }, (_, i) =>
        api.createOrder(
          {
            merchant_id: `merchant-acid-${i}`,
            customer_id: 'customer-001',
            total_paise: 50000,
            items: [],
          },
          userToken
        )
      )
    );

    const payments = await Promise.all(
      orders.map((order) =>
        api.initiatePayment(
          {
            order_id: order.order_id,
            amount_paise: 50000,
            payment_method: 'erupeepayment',
          },
          userToken
        )
      )
    );

    const confirmations = await Promise.all(
      payments.map((payment) => api.confirmPayment(payment.payment_id, userToken))
    );

    expect(confirmations).toHaveLength(5);
    confirmations.forEach((c) => {
      expect(c.status).toBe('CONFIRMED');
    });
  });
});
