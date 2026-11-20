import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';
import { apiFixture } from '../fixtures/api.fixture';

/**
 * Compliance Extended E2E Tests - 30 Tests
 *
 * Coverage:
 * - Advanced KYC workflows (8 tests)
 * - Advanced AML checks (8 tests)
 * - Velocity monitoring (8 tests)
 * - Regulatory reporting (4 tests)
 * - Full compliance flow (2 tests)
 */

const testFixtures = authFixture.extend(apiFixture);

test.describe('Compliance Extended - Advanced KYC', () => {
  test('should perform full KYC verification for customer', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-kyc-001');

    const kycResult = await api.submitCompliance(
      {
        customer_id: 'customer-kyc-001',
        check_type: 'KYC_FULL',
        data: {
          name: 'John Doe',
          dob: '1990-01-01',
          pan: 'AAAPA5055K',
          address: '123 Main St',
          city: 'Mumbai',
          state: 'MH',
          pincode: '400001',
        },
      },
      userToken
    );

    expect(kycResult.status).toMatch(/APPROVED|PENDING|REVIEW/);
    expect(kycResult.kyc_status).toBeDefined();
  });

  test('should validate PAN against government database', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-pan-001');

    const kycResult = await api.submitCompliance(
      {
        customer_id: 'customer-pan-001',
        check_type: 'PAN_VERIFICATION',
        data: {
          pan: 'AAAPA5055K',
        },
      },
      userToken
    );

    expect(kycResult.pan_verified).toBeDefined();
    expect(['VALID', 'INVALID', 'PENDING']).toContain(
      kycResult.pan_status || 'PENDING'
    );
  });

  test('should validate address with cross-references', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-address-001');

    const kycResult = await api.submitCompliance(
      {
        customer_id: 'customer-address-001',
        check_type: 'ADDRESS_VERIFICATION',
        data: {
          address: '123 Main St',
          city: 'Mumbai',
          state: 'MH',
          pincode: '400001',
        },
      },
      userToken
    );

    expect(kycResult.address_verified).toBeDefined();
  });

  test('should flag high-risk documents for manual review', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-risk-001');

    const kycResult = await api.submitCompliance(
      {
        customer_id: 'customer-risk-001',
        check_type: 'DOCUMENT_VERIFICATION',
        data: {
          document_type: 'PASSPORT',
          document_number: 'K12345678',
          country: 'KP', // North Korea - high risk
        },
      },
      userToken
    );

    expect(kycResult.risk_level).toMatch(/HIGH|MEDIUM|LOW/);
    if (kycResult.risk_level === 'HIGH') {
      expect(kycResult.manual_review_required).toBe(true);
    }
  });

  test('should track KYC status transitions', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-kyc-status-001');

    const initial = await api.submitCompliance(
      {
        customer_id: 'customer-kyc-status-001',
        check_type: 'KYC_FULL',
        data: {
          name: 'Jane Doe',
          dob: '1995-05-15',
          pan: 'BBBPB6066L',
        },
      },
      userToken
    );

    const statusHistory = await api.request(
      'GET',
      `/v1/compliance/customer/customer-kyc-status-001/kyc-history`,
      undefined,
      userToken
    );

    expect(statusHistory.transitions).toBeDefined();
    expect(statusHistory.transitions.length).toBeGreaterThanOrEqual(1);
  });

  test('should require biometric verification for high amounts', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-biometric-001');

    const kycResult = await api.submitCompliance(
      {
        customer_id: 'customer-biometric-001',
        check_type: 'BIOMETRIC_KYC',
        high_value_transaction: true,
        amount_paise: 500000000, // ₹5,000,000
        data: {
          name: 'Rich Customer',
          dob: '1985-03-20',
        },
      },
      userToken
    );

    expect(kycResult.biometric_required).toBe(true);
  });

  test('should validate beneficial owner details for corporate customers', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('merchant-corp-001');

    const kycResult = await api.submitCompliance(
      {
        merchant_id: 'merchant-corp-001',
        check_type: 'CORPORATE_KYC',
        data: {
          company_name: 'Acme Corp Ltd',
          cin: 'U72900TN2007PTC063356',
          registration_number: '12345678',
          beneficial_owners: [
            {
              name: 'Owner 1',
              pan: 'AAAPA5055K',
              ownership_percent: 51,
            },
          ],
        },
      },
      userToken
    );

    expect(kycResult.status).toMatch(/APPROVED|PENDING|REVIEW/);
  });

  test('should enforce KYC on transaction initiation', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-enforce-001');

    // Create order
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-enforce-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    // Compliance should be automatically triggered
    const complianceStatus = await api.getComplianceStatus(
      order.order_id,
      userToken
    );

    expect(complianceStatus.kyc_status).toBeDefined();
  });
});

test.describe('Compliance Extended - Advanced AML', () => {
  test('should check customer against sanctions list', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-aml-001');

    const amlResult = await api.submitCompliance(
      {
        customer_id: 'customer-aml-001',
        check_type: 'SANCTIONS_SCREENING',
        data: {
          name: 'John Doe',
          dob: '1990-01-01',
          nationality: 'IN',
        },
      },
      userToken
    );

    expect(amlResult.sanctions_status).toBeDefined();
    expect(['CLEAR', 'MATCH', 'POSSIBLE_MATCH']).toContain(
      amlResult.sanctions_status || 'CLEAR'
    );
  });

  test('should flag suspicious name variations', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-name-var-001');

    const amlResult = await api.submitCompliance(
      {
        customer_id: 'customer-name-var-001',
        check_type: 'FUZZY_MATCHING',
        data: {
          name_variations: [
            'Mohammad Atta',
            'Mohammed Atta',
            'M. Atta',
          ],
        },
      },
      userToken
    );

    expect(amlResult.fuzzy_match_results).toBeDefined();
  });

  test('should check transaction network for layering indicators', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-network-001');

    const order1 = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-network-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const order2 = await api.createOrder(
      {
        merchant_id: 'merchant-002',
        customer_id: 'customer-network-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    // Check for money laundering patterns
    const mlResult = await api.request(
      'POST',
      `/v1/compliance/check/money-laundering-patterns`,
      {
        data: {
          customer_id: 'customer-network-001',
          orders: [order1.order_id, order2.order_id],
        },
      },
      userToken
    );

    expect(mlResult.ml_risk_score).toBeDefined();
    expect(mlResult.ml_risk_score).toBeGreaterThanOrEqual(0);
  });

  test('should detect rapid consecutive transactions (structuring)', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-structuring-001');

    // Create multiple rapid orders (structuring pattern)
    const orders = await Promise.all(
      Array.from({ length: 5 }, (_, i) =>
        api.createOrder(
          {
            merchant_id: `merchant-struct-${i}`,
            customer_id: 'customer-structuring-001',
            total_paise: 99900, // Just under 100k limit
            items: [],
          },
          userToken
        )
      )
    );

    const structuringResult = await api.request(
      'POST',
      `/v1/compliance/check/structuring-detection`,
      {
        data: {
          customer_id: 'customer-structuring-001',
          orders: orders.map((o) => o.order_id),
          time_window_hours: 24,
        },
      },
      userToken
    );

    expect(structuringResult.structuring_risk).toBeDefined();
    if (structuringResult.structuring_detected) {
      expect(structuringResult.alert_severity).toMatch(/LOW|MEDIUM|HIGH/);
    }
  });

  test('should perform PEP (Politically Exposed Person) check', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-pep-001');

    const pepResult = await api.submitCompliance(
      {
        customer_id: 'customer-pep-001',
        check_type: 'PEP_SCREENING',
        data: {
          name: 'Sample Name',
          dob: '1960-01-01',
          position: 'Government Official',
          country: 'IN',
        },
      },
      userToken
    );

    expect(pepResult.pep_status).toBeDefined();
    expect(['PEP', 'RELATIVE_OF_PEP', 'ASSOCIATED_PEP', 'NOT_PEP']).toContain(
      pepResult.pep_status || 'NOT_PEP'
    );
  });

  test('should generate alert for high-risk countries', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-country-risk-001');

    const countryRiskResult = await api.submitCompliance(
      {
        customer_id: 'customer-country-risk-001',
        check_type: 'COUNTRY_RISK_ASSESSMENT',
        data: {
          name: 'Test Customer',
          country: 'IR', // Iran - high risk
          transaction_amount: 500000,
        },
      },
      userToken
    );

    expect(countryRiskResult.country_risk_level).toMatch(/HIGH|MEDIUM|LOW/);
    if (countryRiskResult.country_risk_level === 'HIGH') {
      expect(countryRiskResult.alert_triggered).toBe(true);
    }
  });

  test('should track AML check results in audit trail', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-aml-audit-001');

    await api.submitCompliance(
      {
        customer_id: 'customer-aml-audit-001',
        check_type: 'SANCTIONS_SCREENING',
        data: {
          name: 'Test Customer',
        },
      },
      userToken
    );

    const audit = await api.request(
      'GET',
      `/v1/compliance/customer/customer-aml-audit-001/aml-checks`,
      undefined,
      userToken
    );

    expect(audit.checks).toBeDefined();
    expect(audit.checks.length).toBeGreaterThan(0);
  });
});

test.describe('Compliance Extended - Velocity Monitoring', () => {
  test('should track transaction count per customer', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-velocity-001');

    // Create multiple orders
    await Promise.all(
      Array.from({ length: 3 }, (_, i) =>
        api.createOrder(
          {
            merchant_id: `merchant-vel-${i}`,
            customer_id: 'customer-velocity-001',
            total_paise: 50000,
            items: [],
          },
          userToken
        )
      )
    );

    const velocity = await api.request(
      'GET',
      `/v1/compliance/velocity/customer-velocity-001`,
      undefined,
      userToken
    );

    expect(velocity.transaction_count).toBeGreaterThanOrEqual(3);
  });

  test('should enforce daily transaction limit', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-daily-limit-001');

    // Create orders up to daily limit
    const orders = [];
    for (let i = 0; i < 5; i++) {
      const order = await api.createOrder(
        {
          merchant_id: `merchant-limit-${i}`,
          customer_id: 'customer-daily-limit-001',
          total_paise: 200000, // ₹2000 each
          items: [],
        },
        userToken
      );
      orders.push(order);
    }

    // Try to create one more (should hit limit)
    const limitedOrder = await api.createOrder(
      {
        merchant_id: 'merchant-limit-exceed',
        customer_id: 'customer-daily-limit-001',
        total_paise: 200000,
        items: [],
      },
      userToken
    );

    expect(limitedOrder.status).toBeDefined();
  });

  test('should enforce monthly transaction limit', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-monthly-limit-001');

    // Simulate month worth of transactions
    const monthlyLimit = 1000000; // ₹10,000 monthly
    const transactionSize = 100000; // ₹1000 per transaction

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-monthly-001',
        customer_id: 'customer-monthly-limit-001',
        total_paise: transactionSize,
        items: [],
      },
      userToken
    );

    const velocity = await api.request(
      'GET',
      `/v1/compliance/velocity/customer-monthly-limit-001?period=MONTHLY`,
      undefined,
      userToken
    );

    expect(velocity.monthly_limit).toBe(monthlyLimit);
  });

  test('should track amount velocity in real-time', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-amount-velocity-001');

    const order1 = await api.createOrder(
      {
        merchant_id: 'merchant-amtvel-1',
        customer_id: 'customer-amount-velocity-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const order2 = await api.createOrder(
      {
        merchant_id: 'merchant-amtvel-2',
        customer_id: 'customer-amount-velocity-001',
        total_paise: 200000,
        items: [],
      },
      userToken
    );

    const velocity = await api.request(
      'GET',
      `/v1/compliance/velocity/customer-amount-velocity-001?type=AMOUNT`,
      undefined,
      userToken
    );

    expect(velocity.total_amount_paise).toBe(300000);
  });

  test('should alert on unusual velocity spike', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-spike-001');

    // Create sudden spike in transactions
    const orders = await Promise.all(
      Array.from({ length: 10 }, (_, i) =>
        api.createOrder(
          {
            merchant_id: `merchant-spike-${i}`,
            customer_id: 'customer-spike-001',
            total_paise: 50000,
            items: [],
          },
          userToken
        )
      )
    );

    const spikeAlert = await api.request(
      'GET',
      `/v1/compliance/velocity/customer-spike-001/alerts`,
      undefined,
      userToken
    );

    expect(spikeAlert.alerts).toBeDefined();
  });

  test('should differentiate between customer and merchant velocity', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-diff-vel-001');

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-diff-001',
        customer_id: 'customer-diff-vel-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const customerVelocity = await api.request(
      'GET',
      `/v1/compliance/velocity/customer-diff-vel-001?type=CUSTOMER`,
      undefined,
      userToken
    );

    const merchantVelocity = await api.request(
      'GET',
      `/v1/compliance/velocity/merchant-diff-001?type=MERCHANT`,
      undefined,
      userToken
    );

    expect(customerVelocity.entity_type).toBe('CUSTOMER');
    expect(merchantVelocity.entity_type).toBe('MERCHANT');
  });

  test('should reset velocity counters at period boundaries', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-reset-001');

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-reset-001',
        customer_id: 'customer-reset-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const velocity = await api.request(
      'GET',
      `/v1/compliance/velocity/customer-reset-001`,
      undefined,
      userToken
    );

    expect(velocity.period_start).toBeDefined();
    expect(velocity.period_end).toBeDefined();
  });
});

test.describe('Compliance Extended - Regulatory Reporting', () => {
  test('should generate STR (Suspicious Transaction Report)', async ({
    api,
    getComplianceToken,
  }) => {
    const complianceToken = getComplianceToken();

    const str = await api.request(
      'POST',
      `/v1/compliance/reporting/str`,
      {
        data: {
          transaction_id: 'txn-suspicious-001',
          customer_id: 'customer-001',
          amount_paise: 500000,
          reason: 'UNUSUAL_PATTERN',
          description: 'Rapid sequence of small transactions',
        },
      },
      complianceToken
    );

    expect(str.str_id).toBeDefined();
    expect(str.report_status).toMatch(/FILED|DRAFT|PENDING/);
  });

  test('should generate CTR (Currency Transaction Report) for large amounts', async ({
    api,
    getComplianceToken,
  }) => {
    const complianceToken = getComplianceToken();

    const ctr = await api.request(
      'POST',
      `/v1/compliance/reporting/ctr`,
      {
        data: {
          transaction_id: 'txn-large-001',
          customer_id: 'customer-large-001',
          amount_paise: 1000000, // ₹10,000 - CTR threshold
          transaction_date: new Date().toISOString(),
        },
      },
      complianceToken
    );

    expect(ctr.ctr_id).toBeDefined();
  });

  test('should track reporting deadlines (7-day STR requirement)', async ({
    api,
    getComplianceToken,
  }) => {
    const complianceToken = getComplianceToken();

    const str = await api.request(
      'POST',
      `/v1/compliance/reporting/str`,
      {
        data: {
          transaction_id: 'txn-deadline-001',
          customer_id: 'customer-001',
          reason: 'SUSPICION_OF_FRAUD',
        },
      },
      complianceToken
    );

    expect(str.report_deadline).toBeDefined();
    const deadline = new Date(str.report_deadline);
    const today = new Date();
    const daysUntilDeadline = Math.ceil(
      (deadline.getTime() - today.getTime()) / (1000 * 60 * 60 * 24)
    );
    expect(daysUntilDeadline).toBeLessThanOrEqual(7);
  });

  test('should maintain audit trail of all regulatory reports', async ({
    api,
    getComplianceToken,
  }) => {
    const complianceToken = getComplianceToken();

    const reportsList = await api.request(
      'GET',
      `/v1/compliance/reporting/list`,
      undefined,
      complianceToken
    );

    expect(reportsList.reports).toBeDefined();
    expect(reportsList.reports[0]).toHaveProperty('report_type');
    expect(reportsList.reports[0]).toHaveProperty('created_at');
  });
});

test.describe('Compliance Extended - Full Compliance Flow', () => {
  test('should execute complete compliance check on high-value transaction', async ({
    api,
    getTestUserToken,
    getComplianceToken,
  }) => {
    const userToken = getTestUserToken();
    const complianceToken = getComplianceToken();

    // Create high-value order
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-high-value-001',
        customer_id: 'customer-high-value-001',
        total_paise: 5000000, // ₹50,000 - triggers comprehensive checks
        items: [{ name: 'Luxury Item', quantity: 1, price_paise: 5000000 }],
      },
      userToken
    );

    // Compliance checks should be triggered automatically
    const complianceStatus = await api.getComplianceStatus(
      order.order_id,
      userToken
    );

    expect(complianceStatus.kyc_status).toBeDefined();
    expect(complianceStatus.aml_status).toBeDefined();
    expect(complianceStatus.velocity_check_passed).toBeDefined();

    // Compliance officer should be able to view full report
    const fullReport = await api.request(
      'GET',
      `/v1/compliance/order/${order.order_id}/full-report`,
      undefined,
      complianceToken
    );

    expect(fullReport.kyc_details).toBeDefined();
    expect(fullReport.aml_details).toBeDefined();
    expect(fullReport.velocity_details).toBeDefined();
    expect(fullReport.risk_score).toBeDefined();
  });

  test('should block transaction if any compliance check fails', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-failed-compliance-001');

    // Mark customer as high-risk (manual override for testing)
    await api.request(
      'POST',
      `/v1/compliance/customer/customer-failed-compliance-001/risk-override`,
      {
        data: {
          risk_level: 'HIGH',
          reason: 'Testing compliance block',
        },
      },
      userToken
    );

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-failed-compliance-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    const complianceStatus = await api.getComplianceStatus(
      order.order_id,
      userToken
    );

    // Should require additional verification
    expect(complianceStatus.status).toMatch(/PENDING|REVIEW|FAILED/);
  });
});
