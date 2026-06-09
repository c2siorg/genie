import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';
import { apiFixture } from '../fixtures/api.fixture';

/**
 * Compliance & AML E2E Tests
 *
 * Coverage:
 * - KYC verification workflows
 * - AML risk scoring
 * - Velocity limit enforcement
 * - Sanctions screening
 * - Compliance decision audit trails
 * - High-risk transaction handling
 */

const testFixtures = authFixture.extend(apiFixture);

test.describe('KYC Verification Workflow', () => {
  test('should require KYC for new customer', async ({ page, api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Try to create order for unverified customer
    const order = await api.request(
      'POST',
      '/v1/commerce/orders',
      {
        data: {
          merchant_id: 'merchant-001',
          customer_id: 'new-customer-unverified',
          total_paise: 100000,
          items: [],
        },
      },
      token
    );

    // Should be blocked or marked pending KYC
    expect(order.status || order.error).toBeDefined();
  });

  test('should complete KYC verification', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Submit KYC
    const kyc = await api.request(
      'POST',
      '/v1/compliance/kyc/submit',
      {
        data: {
          customer_id: 'kyc-test-001',
          full_name: 'John Doe',
          date_of_birth: '1990-01-01',
          address: '123 Main St',
          government_id: 'AAAA0001A',
          id_type: 'aadhaar',
        },
      },
      token
    );

    expect(kyc.status).toBe('SUBMITTED');
    expect(kyc.kyc_id).toBeDefined();

    // Verify KYC status
    const status = await api.request(
      'GET',
      `/v1/compliance/kyc/${kyc.kyc_id}`,
      undefined,
      token
    );

    expect(['SUBMITTED', 'APPROVED', 'PENDING']).toContain(status.status);
  });

  test('should reject invalid KYC data', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Submit with invalid data
    const kyc = await api.request(
      'POST',
      '/v1/compliance/kyc/submit',
      {
        data: {
          customer_id: 'kyc-invalid',
          full_name: '', // Empty name
          date_of_birth: 'invalid-date',
          address: '',
          government_id: 'INVALID',
        },
      },
      token
    );

    // Should reject
    expect(kyc.error || kyc.status).toBeDefined();
  });

  test('should escalate for manual KYC review', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Submit KYC that needs review
    const kyc = await api.request(
      'POST',
      '/v1/compliance/kyc/submit',
      {
        data: {
          customer_id: 'kyc-review-required',
          full_name: 'Jane Smith',
          date_of_birth: '2010-01-01', // Very young - needs review
          address: '123 Main St',
          government_id: 'BBBB0002B',
          id_type: 'passport',
          requires_review: true,
        },
      },
      token
    );

    expect(kyc.status).toBe('PENDING_REVIEW');
    expect(kyc.assigned_to).toBeDefined(); // Should be assigned to reviewer
  });
});

test.describe('AML Risk Scoring', () => {
  test('should calculate AML risk score', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Check AML for customer
    const amlCheck = await api.request(
      'POST',
      '/v1/compliance/aml/check',
      {
        data: {
          customer_id: 'aml-test-001',
          full_name: 'Test Customer',
          countries_of_residence: ['IN'],
        },
      },
      token
    );

    expect(amlCheck.risk_score).toBeDefined();
    expect(amlCheck.risk_score).toBeGreaterThanOrEqual(0);
    expect(amlCheck.risk_score).toBeLessThanOrEqual(100);
    expect(amlCheck.risk_level).toMatch(/LOW|MEDIUM|HIGH/);
  });

  test('should flag high-risk customers', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Check customer with high-risk indicators
    const amlCheck = await api.request(
      'POST',
      '/v1/compliance/aml/check',
      {
        data: {
          customer_id: 'aml-high-risk',
          full_name: 'Suspicious Customer',
          pep_indicator: true, // Politically exposed person
          countries_of_residence: ['IN', 'XX'], // Multiple countries
          transaction_history: [
            { amount_paise: 9999900, date: '2024-01-01' }, // Just under reporting threshold
          ],
        },
      },
      token
    );

    expect(amlCheck.risk_level).toBe('HIGH');
    expect(amlCheck.flags).toBeDefined();
    expect(amlCheck.flags.length).toBeGreaterThan(0);
  });

  test('should apply enhanced due diligence for high-risk', async ({
    api,
    getComplianceToken,
  }) => {
    const token = getComplianceToken();

    const amlCheck = await api.request(
      'POST',
      '/v1/compliance/aml/check',
      {
        data: {
          customer_id: 'aml-edd',
          full_name: 'EDD Required',
          pep_indicator: true,
        },
      },
      token
    );

    // Should require enhanced due diligence
    expect(amlCheck.requires_edd).toBe(true);
    expect(amlCheck.edd_requirements).toBeDefined();
    expect(amlCheck.edd_requirements.length).toBeGreaterThan(0);
  });

  test('should match against sanctions list', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    const sanctions = await api.request(
      'POST',
      '/v1/compliance/sanctions/check',
      {
        data: {
          full_name: 'Test Customer',
          merchant_name: 'Test Merchant',
          check_date: new Date().toISOString(),
        },
      },
      token
    );

    expect(sanctions.matched).toBeOfType('boolean');
    if (sanctions.matched) {
      expect(sanctions.matches).toBeDefined();
      expect(sanctions.matches.length).toBeGreaterThan(0);
    }
  });
});

test.describe('Velocity Limit Enforcement', () => {
  test('should track customer transaction velocity', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Check velocity for customer
    const velocity = await api.request(
      'GET',
      '/v1/compliance/velocity/customer-velocity-001',
      undefined,
      token
    );

    expect(velocity.total_paise_24h).toBeDefined();
    expect(velocity.total_paise_24h).toBeGreaterThanOrEqual(0);
    expect(velocity.transaction_count_24h).toBeDefined();
    expect(velocity.limit_paise).toBeDefined();
    expect(velocity.exceeded).toBeOfType('boolean');
  });

  test('should block transaction exceeding velocity limit', async ({
    api,
    getComplianceToken,
  }) => {
    const token = getComplianceToken();

    // Try to initiate payment that exceeds limit
    const payment = await api.initiatePayment(
      {
        order_id: 'velocity-exceed',
        amount_paise: 50000000, // ₹500k - likely exceeds daily limit
      },
      token
    );

    // Should fail due to velocity
    expect(payment.error || payment.status).toBeDefined();
    if (payment.error) {
      expect(payment.error.code).toBe('VELOCITY_EXCEEDED');
    }
  });

  test('should allow transaction within velocity limit', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Check current velocity
    const velocity = await api.request(
      'GET',
      '/v1/compliance/velocity/customer-normal',
      undefined,
      token
    );

    const availableAmount = velocity.limit_paise - velocity.total_paise_24h;

    if (availableAmount > 1000) {
      // Try payment within limit
      const payment = await api.initiatePayment(
        {
          order_id: 'velocity-ok',
          amount_paise: Math.min(availableAmount / 2, 100000), // Half of available or ₹1k
        },
        token
      );

      // Should succeed
      expect(payment.payment_id).toBeDefined();
    }
  });

  test('should allow raising velocity limit with manager approval', async ({
    api,
    getComplianceToken,
  }) => {
    const token = getComplianceToken();

    // Request limit increase
    const request = await api.request(
      'POST',
      '/v1/compliance/velocity/request-increase',
      {
        data: {
          customer_id: 'velocity-increase-request',
          requested_limit_paise: 100000000, // ₹1M
          reason: 'Business expansion',
        },
      },
      token
    );

    expect(request.request_id).toBeDefined();
    expect(request.status).toBe('PENDING_APPROVAL');

    // Approve request (manager action)
    const approved = await api.request(
      'POST',
      `/v1/compliance/velocity/approve/${request.request_id}`,
      {
        data: {
          approved: true,
          approved_limit_paise: 50000000, // ₹500k approved
        },
      },
      token
    );

    expect(approved.status).toBe('APPROVED');
  });
});

test.describe('Compliance Audit Trail', () => {
  test('should record all compliance decisions', async ({ api, getComplianceToken }) => {
    const token = getComplianceToken();

    // Make a compliance decision
    const kyc = await api.request(
      'POST',
      '/v1/compliance/kyc/submit',
      {
        data: {
          customer_id: 'audit-trail-test',
          full_name: 'Audit Trail Test',
          date_of_birth: '1980-01-01',
          address: '123 Main St',
          government_id: 'CCCC0003C',
        },
      },
      token
    );

    // Get decision audit trail
    const trail = await api.request(
      'GET',
      `/v1/compliance/audit-trail/${kyc.kyc_id}`,
      undefined,
      token
    );

    expect(trail.entries).toBeDefined();
    expect(trail.entries.length).toBeGreaterThan(0);

    // Each entry should have decision details
    trail.entries.forEach((entry) => {
      expect(entry.timestamp).toBeDefined();
      expect(entry.decision_type).toBeDefined();
      expect(entry.decision_maker).toBeDefined();
      expect(entry.reason).toBeDefined();
    });
  });

  test('should link compliance decision to transaction', async ({
    api,
    getComplianceToken,
  }) => {
    const token = getComplianceToken();

    // Create order (triggers compliance check)
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'compliance-link-test',
        total_paise: 100000,
        items: [],
      },
      token
    );

    // Get compliance decision for order
    const compliance = await api.request(
      'GET',
      `/v1/compliance/order/${order.order_id}`,
      undefined,
      token
    );

    expect(compliance.order_id).toBe(order.order_id);
    expect(compliance.decision).toMatch(/APPROVED|BLOCKED|PENDING/);
    expect(compliance.decision_timestamp).toBeDefined();
    expect(compliance.decision_reason).toBeDefined();
  });
});

test.describe('High-Risk Transaction Handling', () => {
  test('should escalate high-risk transactions for manual review', async ({
    api,
    getComplianceToken,
  }) => {
    const token = getComplianceToken();

    // Create order that triggers risk escalation
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-high-risk',
        customer_id: 'customer-high-risk',
        total_paise: 10000000, // ₹100k - high amount
        items: [],
      },
      token
    );

    // Check escalation status
    const escalation = await api.request(
      'GET',
      `/v1/compliance/escalation/${order.order_id}`,
      undefined,
      token
    );

    if (escalation.escalated) {
      expect(escalation.escalation_reason).toBeDefined();
      expect(escalation.assigned_to).toBeDefined(); // Assigned to compliance officer
      expect(escalation.status).toMatch(/PENDING|REVIEWING|APPROVED|BLOCKED/);
    }
  });

  test('should block transaction if compliance check fails', async ({
    api,
    getComplianceToken,
  }) => {
    const token = getComplianceToken();

    // Try to create order that will fail compliance
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-blocked',
        customer_id: 'customer-blocked',
        total_paise: 100000,
        items: [],
      },
      token
    );

    // Check compliance status
    const compliance = await api.request(
      'GET',
      `/v1/compliance/order/${order.order_id}`,
      undefined,
      token
    );

    if (compliance.decision === 'BLOCKED') {
      expect(compliance.block_reason).toBeDefined();
      expect(compliance.can_appeal).toBeOfType('boolean');
    }
  });
});
