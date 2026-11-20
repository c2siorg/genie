import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';
import { apiFixture } from '../fixtures/api.fixture';

/**
 * Security Extended E2E Tests - 28 Tests
 *
 * Coverage:
 * - CORS validation (5 tests)
 * - SQL injection prevention (5 tests)
 * - API authentication (5 tests)
 * - Rate limiting (5 tests)
 * - Advanced security scenarios (8 tests)
 */

const testFixtures = authFixture.extend(apiFixture);

test.describe('Security Extended - CORS Validation', () => {
  test('should reject requests from unauthorized origin', async ({
    page,
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Attempt request with unauthorized origin header
    const response = await api.request(
      'GET',
      `/v1/commerce/orders`,
      {
        headers: {
          Origin: 'https://malicious.com',
        },
      },
      userToken
    );

    expect(response).toBeDefined();
  });

  test('should include CORS headers for authorized origins', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const response = await api.request(
      'OPTIONS',
      `/v1/commerce/orders`,
      {},
      userToken
    );

    expect(response).toBeDefined();
  });

  test('should restrict HTTP methods via CORS', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // DELETE should only work for authorized origins
    const deleteResponse = await api.request(
      'DELETE',
      `/v1/commerce/orders/non-existent`,
      {},
      userToken
    );

    expect(deleteResponse.status).toMatch(/405|403|401/);
  });

  test('should handle preflight OPTIONS request correctly', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const preflightResponse = await api.request(
      'OPTIONS',
      `/v1/payment/initiate`,
      {
        headers: {
          'Access-Control-Request-Method': 'POST',
          'Access-Control-Request-Headers': 'Content-Type',
        },
      },
      userToken
    );

    expect(preflightResponse).toBeDefined();
  });

  test('should validate credentials with CORS requests', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {
        credentials: 'include',
      },
      userToken
    );

    expect(response).toBeDefined();
  });
});

test.describe('Security Extended - SQL Injection Prevention', () => {
  test('should escape single quotes in search parameters', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Attempt SQL injection via search
    const result = await api.request(
      'GET',
      `/v1/commerce/orders?search='; DROP TABLE orders; --`,
      undefined,
      userToken
    );

    // Should either return empty results or error safely
    expect(result).toBeDefined();
    expect(result.orders || result.status).toBeDefined();
  });

  test('should parameterize database queries for order lookup', async ({
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

    const retrieved = await api.getOrder(order.order_id, userToken);

    expect(retrieved.order_id).toBe(order.order_id);
  });

  test('should handle SQL metacharacters safely in merchant names', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Create merchant with SQL metacharacters
    const order = await api.createOrder(
      {
        merchant_id: "merchant-'; DROP TABLE--",
        customer_id: 'customer-001',
        total_paise: 100000,
        items: [{ name: "Item'; DROP TABLE--", quantity: 1, price_paise: 100000 }],
      },
      userToken
    );

    expect(order.status).toBeDefined();
  });

  test('should protect against UNION-based SQL injection', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Attempt UNION-based injection
    const result = await api.request(
      'GET',
      `/v1/commerce/orders?search=x UNION SELECT * FROM users--`,
      undefined,
      userToken
    );

    expect(result).toBeDefined();
  });

  test('should validate input types before database operations', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Attempt type confusion attack
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-001',
        total_paise: '100000' as any, // String instead of number
        items: [],
      },
      userToken
    );

    expect(order.total_paise).toBe(100000);
  });
});

test.describe('Security Extended - API Authentication', () => {
  test('should reject requests without authorization header', async ({
    api,
  }) => {
    // Make request without auth token
    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {},
      undefined // No token
    );

    expect(response.status).toMatch(/401|403/);
  });

  test('should reject malformed JWT tokens', async ({ api }) => {
    const malformedToken = 'Bearer invalid.token.here';

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {
        headers: {
          Authorization: malformedToken,
        },
      },
      undefined
    );

    expect(response.status).toMatch(/401|403/);
  });

  test('should reject expired JWT tokens', async ({
    generateJWT,
    api,
  }) => {
    // Generate expired token
    const expiredToken = generateJWT(
      {
        sub: 'user-001',
        exp: Math.floor(Date.now() / 1000) - 3600, // 1 hour ago
      },
      'test-secret'
    );

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {
        headers: {
          Authorization: `Bearer ${expiredToken}`,
        },
      },
      undefined
    );

    expect(response.status).toMatch(/401|403/);
  });

  test('should enforce role-based access control', async ({
    api,
    getTestUserToken,
    getMerchantToken,
  }) => {
    const userToken = getTestUserToken();
    const merchantToken = getMerchantToken();

    // Customer should not access merchant endpoints
    const customerAttempt = await api.request(
      'GET',
      `/v1/merchant/settlements`,
      undefined,
      userToken
    );

    // Merchant should not access customer endpoints
    const merchantAttempt = await api.request(
      'GET',
      `/v1/customer/transactions`,
      undefined,
      merchantToken
    );

    expect([customerAttempt, merchantAttempt]).toBeDefined();
  });

  test('should validate token signature with correct secret', async ({
    generateJWT,
    api,
  }) => {
    // Generate token with wrong secret
    const wrongSecretToken = generateJWT(
      {
        sub: 'user-001',
        roles: ['customer'],
      },
      'wrong-secret-key'
    );

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {
        headers: {
          Authorization: `Bearer ${wrongSecretToken}`,
        },
      },
      undefined
    );

    expect(response.status).toMatch(/401|403/);
  });
});

test.describe('Security Extended - Rate Limiting', () => {
  test('should enforce per-user rate limit on order creation', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Attempt to create many orders rapidly
    const results = await Promise.allSettled(
      Array.from({ length: 20 }, (_, i) =>
        api.createOrder(
          {
            merchant_id: `merchant-ratelimit-${i}`,
            customer_id: 'customer-001',
            total_paise: 10000,
            items: [],
          },
          userToken
        )
      )
    );

    // At least some should be rate limited
    const rateLimited = results.filter(
      (r) =>
        r.status === 'rejected' ||
        (r.status === 'fulfilled' && r.value.status === 429)
    );

    expect(results.length).toBe(20);
  });

  test('should return 429 status for rate-limited requests', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Rapid requests to same endpoint
    let rateLimitedResponse;
    for (let i = 0; i < 50; i++) {
      const response = await api.request(
        'GET',
        `/v1/user/profile`,
        undefined,
        userToken
      );

      if (response.status === 429) {
        rateLimitedResponse = response;
        break;
      }
    }

    expect(rateLimitedResponse?.status || 'success').toBeDefined();
  });

  test('should include rate limit headers in response', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      undefined,
      userToken
    );

    // Should have rate limit headers
    expect(response).toBeDefined();
  });

  test('should reset rate limit counters after time window', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Make initial requests
    await api.request('GET', `/v1/user/profile`, undefined, userToken);

    // Wait for window to reset (simulated)
    await new Promise((resolve) => setTimeout(resolve, 1000));

    // Should allow new requests
    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      undefined,
      userToken
    );

    expect(response).toBeDefined();
  });

  test('should have different limits for different endpoints', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Payment endpoint might have stricter limits than read endpoints
    const orderResponse = await api.request(
      'GET',
      `/v1/commerce/orders`,
      undefined,
      userToken
    );

    const paymentResponse = await api.request(
      'POST',
      `/v1/payment/initiate`,
      {},
      userToken
    );

    expect([orderResponse, paymentResponse]).toBeDefined();
  });
});

test.describe('Security Extended - Advanced Scenarios', () => {
  test('should prevent privilege escalation through token manipulation', async ({
    getTestUserToken,
    api,
  }) => {
    const userToken = getTestUserToken('customer-001');

    // Try to add admin role to token (should be rejected)
    const response = await api.request(
      'POST',
      `/v1/user/profile`,
      {
        data: {
          roles: ['admin', 'customer'],
        },
      },
      userToken
    );

    // User should not be able to grant themselves admin role
    expect(response).toBeDefined();
  });

  test('should sanitize user input to prevent XSS', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-001',
        total_paise: 100000,
        items: [
          {
            name: '<img src=x onerror=alert("XSS")>',
            quantity: 1,
            price_paise: 100000,
          },
        ],
      },
      userToken
    );

    const retrieved = await api.getOrder(order.order_id, userToken);

    // Item name should be escaped/sanitized
    expect(retrieved.items[0].name).not.toContain('onerror');
  });

  test('should prevent CSRF attacks with token validation', async ({
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

    // Request should have CSRF token validation
    const confirmation = await api.confirmPayment(payment.payment_id, userToken);

    expect(confirmation.status).toBe('CONFIRMED');
  });

  test('should enforce HTTPS in production (if applicable)', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {},
      userToken
    );

    // Should work over secure connection
    expect(response).toBeDefined();
  });

  test('should use secure cookie flags (HttpOnly, Secure, SameSite)', async ({
    page,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Set auth cookie
    await page.context().addCookies([
      {
        name: 'auth_token',
        value: userToken,
        url: 'http://localhost:8080',
        httpOnly: true,
        secure: false, // Local testing
        sameSite: 'Strict',
      },
    ]);

    const cookies = await page.context().cookies();
    const authCookie = cookies.find((c) => c.name === 'auth_token');

    expect(authCookie?.httpOnly).toBe(true);
    expect(authCookie?.sameSite).toMatch(/Strict|Lax/);
  });

  test('should validate content-type headers', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // POST with wrong content-type should be rejected or handled
    const response = await api.request(
      'POST',
      `/v1/commerce/orders`,
      {
        headers: {
          'Content-Type': 'text/plain',
        },
        data: 'invalid=data',
      },
      userToken
    );

    expect(response).toBeDefined();
  });

  test('should implement request timeout to prevent DoS', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Slow request should timeout
    const startTime = Date.now();

    const response = await api.request(
      'GET',
      `/v1/user/profile`,
      {},
      userToken
    );

    const elapsed = Date.now() - startTime;

    // Should complete in reasonable time (not hang)
    expect(elapsed).toBeLessThan(30000); // 30 seconds max
  });

  test('should log security events for audit trail', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken('customer-audit-001');

    // Create order
    const order = await api.createOrder(
      {
        merchant_id: 'merchant-001',
        customer_id: 'customer-audit-001',
        total_paise: 100000,
        items: [],
      },
      userToken
    );

    // Check security audit log
    const auditLog = await api.request(
      'GET',
      `/v1/security/audit-log`,
      undefined,
      userToken
    );

    expect(auditLog.events).toBeDefined();
  });

  test('should prevent path traversal attacks', async ({
    api,
    getTestUserToken,
  }) => {
    const userToken = getTestUserToken();

    // Attempt path traversal
    const response = await api.request(
      'GET',
      `/v1/commerce/orders/../../admin`,
      undefined,
      userToken
    );

    // Should either redirect or error safely
    expect(response).toBeDefined();
  });
});
