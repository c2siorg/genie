import { test, expect } from '@playwright/test';
import { authFixture } from '../fixtures/auth.fixture';

/**
 * Security E2E Tests
 *
 * Coverage:
 * - CSRF token validation
 * - HttpOnly cookie enforcement
 * - Secure cookie flags
 * - XSS protection headers
 * - CORS headers
 * - JWT validation
 * - Session security
 */

const testFixtures = authFixture;

test.describe('CSRF Protection', () => {
  test('should include CSRF token in form', async ({ page }) => {
    await page.goto('/settlement');
    await page.waitForLoadState('networkidle');

    // Check CSRF token is present in hidden input
    const csrfToken = await page.locator('[name="csrf_token"]').inputValue();
    expect(csrfToken).toBeTruthy();
    expect(csrfToken?.length).toBeGreaterThan(20); // Tokens should be substantial
  });

  test('should reject POST without CSRF token', async ({ page, context }) => {
    await page.goto('/settlement');

    // Make POST request without CSRF token
    const response = await context.request.post('http://localhost:8080/v1/settlement/initiate', {
      data: {
        order_id: 'test-001',
        amount: 1000,
        // No csrf_token
      },
    });

    expect(response.status()).toBe(403); // Forbidden
  });

  test('should reject POST with invalid CSRF token', async ({ page, context }) => {
    await page.goto('/settlement');

    // Make POST with wrong token
    const response = await context.request.post('http://localhost:8080/v1/settlement/initiate', {
      data: {
        order_id: 'test-001',
        amount: 1000,
        csrf_token: 'invalid-token-xyz',
      },
    });

    expect(response.status()).toBe(403);
  });

  test('should accept POST with valid CSRF token', async ({ page, context, getTestUserToken }) => {
    await page.goto('/settlement');

    // Get valid CSRF token from page
    const csrfToken = await page.locator('[name="csrf_token"]').inputValue();

    // Make POST with valid token
    const response = await context.request.post('http://localhost:8080/v1/settlement/initiate', {
      data: {
        order_id: 'test-001',
        amount: 1000,
        csrf_token: csrfToken,
      },
      headers: {
        Authorization: `Bearer ${getTestUserToken()}`,
      },
    });

    // Should not be forbidden (may fail for other reasons like invalid order, but not CSRF)
    expect(response.status()).not.toBe(403);
  });

  test('should refresh CSRF token on page reload', async ({ page }) => {
    await page.goto('/settlement');

    const token1 = await page.locator('[name="csrf_token"]').inputValue();

    // Reload page
    await page.reload();
    await page.waitForLoadState('networkidle');

    const token2 = await page.locator('[name="csrf_token"]').inputValue();

    // Token should be different after reload (new session)
    expect(token1).not.toBe(token2);
  });
});

test.describe('Cookie Security', () => {
  test('should set HttpOnly flag on session cookie', async ({ context }) => {
    const cookies = await context.cookies();

    const sessionCookie = cookies.find((c) => c.name === 'session' || c.name.includes('session'));

    expect(sessionCookie).toBeDefined();
    expect(sessionCookie?.httpOnly).toBe(true);
  });

  test('should set Secure flag on cookies over HTTPS', async ({ page }) => {
    // Note: This test assumes HTTPS in production
    // In dev, this may be skipped
    if (process.env.CI || process.env.SECURE_COOKIES_ENFORCED) {
      const context = page.context();
      const cookies = await context.cookies();

      cookies.forEach((cookie) => {
        if (cookie.secure !== undefined) {
          expect(cookie.secure).toBe(true);
        }
      });
    }
  });

  test('should set SameSite=Strict on authentication cookie', async ({ context }) => {
    const cookies = await context.cookies();

    const authCookie = cookies.find((c) => c.name === 'auth' || c.name.includes('auth'));

    expect(authCookie?.sameSite).toBe('Strict');
  });

  test('should not expose cookies to JavaScript', async ({ page }) => {
    await page.goto('/settlement');

    // Try to access cookies from JavaScript
    const jsAccessible = await page.evaluate(() => {
      return document.cookie; // This should be empty or not expose httpOnly cookies
    });

    // Should not include session cookie since it's httpOnly
    expect(jsAccessible).not.toContain('session');
  });

  test('should clear cookies on logout', async ({ page, context }) => {
    await page.goto('/login');

    // Login
    await page.fill('[name="email"]', 'test@example.com');
    await page.fill('[name="password"]', 'password123');
    await page.click('button[type="submit"]');

    await page.waitForURL('/dashboard');

    let cookies = await context.cookies();
    const cookieCountBefore = cookies.length;

    // Logout
    await page.click('[data-testid="logout-btn"]');
    await page.waitForURL('/login');

    cookies = await context.cookies();
    const cookieCountAfter = cookies.length;

    // Should have fewer cookies after logout
    expect(cookieCountAfter).toBeLessThan(cookieCountBefore);
  });
});

test.describe('Security Headers', () => {
  test('should include XSS protection headers', async ({ context }) => {
    const response = await context.request.get('http://localhost:8080/settlement');

    const xssHeader = response.headers()['x-content-type-options'];
    const frameHeader = response.headers()['x-frame-options'];

    expect(xssHeader).toBe('nosniff');
    expect(frameHeader).toMatch(/DENY|SAMEORIGIN/);
  });

  test('should include Content Security Policy header', async ({ context }) => {
    const response = await context.request.get('http://localhost:8080/settlement');

    const cspHeader = response.headers()['content-security-policy'];

    expect(cspHeader).toBeDefined();
    expect(cspHeader).toContain("default-src 'self'");
  });

  test('should set Referrer-Policy header', async ({ context }) => {
    const response = await context.request.get('http://localhost:8080/settlement');

    const refPolicy = response.headers()['referrer-policy'];

    expect(refPolicy).toBe('strict-origin-when-cross-origin');
  });

  test('should not expose server information', async ({ context }) => {
    const response = await context.request.get('http://localhost:8080/settlement');

    const serverHeader = response.headers()['server'];

    // Should not expose Go/net/http version details
    expect(serverHeader).not.toMatch(/Go|net\/http/);
  });
});

test.describe('JWT Token Security', () => {
  test('should reject expired JWT', async ({ page, context, generateJWT }) => {
    const expiredToken = generateJWT(
      {
        sub: 'test-user',
        exp: Math.floor(Date.now() / 1000) - 3600, // Expired 1 hour ago
      },
      'test-secret-key'
    );

    const response = await context.request.get('http://localhost:8080/v1/user/profile', {
      headers: {
        Authorization: `Bearer ${expiredToken}`,
      },
    });

    expect(response.status()).toBe(401);
  });

  test('should reject malformed JWT', async ({ context }) => {
    const response = await context.request.get('http://localhost:8080/v1/user/profile', {
      headers: {
        Authorization: 'Bearer invalid.jwt.token',
      },
    });

    expect(response.status()).toBe(401);
  });

  test('should reject JWT with wrong secret', async ({ context, generateJWT }) => {
    const token = generateJWT({ sub: 'test-user' }, 'wrong-secret');

    const response = await context.request.get('http://localhost:8080/v1/user/profile', {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    expect(response.status()).toBe(401);
  });

  test('should include proper claims in valid JWT', async ({ getTestUserToken }) => {
    const token = getTestUserToken('user-001', ['customer']);

    // Decode token (without verification - just checking structure)
    const parts = token.split('.');
    expect(parts.length).toBe(3); // Header.Payload.Signature

    const payload = JSON.parse(Buffer.from(parts[1], 'base64').toString());

    expect(payload.sub).toBe('user-001');
    expect(payload.roles).toContain('customer');
    expect(payload.iat).toBeDefined();
    expect(payload.exp).toBeDefined();
  });
});

test.describe('Input Validation & XSS Prevention', () => {
  test('should sanitize order ID input', async ({ page }) => {
    await page.goto('/settlement');

    // Try to inject script
    await page.fill('[data-testid="order-id-input"]', '<script>alert("xss")</script>');
    await page.click('[data-testid="initiate-payment-btn"]');

    // Check that script is not executed
    const pageContent = await page.content();
    expect(pageContent).not.toContain('<script>');
  });

  test('should escape special characters in notes', async ({ page, api, getTestUserToken }) => {
    await page.goto('/settlement');

    const maliciousNotes = '<img src=x onerror="alert(\'xss\')">';

    // Submit form with malicious input
    await page.fill('[data-testid="notes-input"]', maliciousNotes);
    await page.click('[data-testid="submit-btn"]');

    // Verify it's stored safely
    const pageContent = await page.content();
    expect(pageContent).not.toContain('onerror=');
  });

  test('should reject SQL injection in search', async ({ page }) => {
    await page.goto('/eval/traces');

    const sqlInjection = "'; DROP TABLE traces; --";

    await page.fill('[data-testid="search-input"]', sqlInjection);
    await page.press('[data-testid="search-input"]', 'Enter');

    // Should still work normally (query safe)
    await page.waitForLoadState('networkidle');

    // Page should still be functional
    expect(await page.isVisible('[data-testid="trace-container"]')).toBeTruthy();
  });
});

test.describe('Rate Limiting', () => {
  test('should rate limit repeated login attempts', async ({ page }) => {
    // Try multiple failed logins
    for (let i = 0; i < 10; i++) {
      await page.goto('/login');
      await page.fill('[name="email"]', 'test@example.com');
      await page.fill('[name="password"]', 'wrongpass');
      await page.click('button[type="submit"]');
      await page.waitForTimeout(100);
    }

    // Should show rate limit message
    const errorMsg = await page.textContent('[data-testid="error-message"]');
    expect(errorMsg).toContain('too many');
  });

  test('should rate limit API requests', async ({ context }) => {
    // Send 100 rapid requests
    const requests = Array.from({ length: 100 }).map(() =>
      context.request.get('http://localhost:8080/v1/user/profile', {
        headers: {
          Authorization: 'Bearer invalid',
        },
      })
    );

    const responses = await Promise.all(requests);

    // Some should get 429 Too Many Requests
    const tooManyRequests = responses.filter((r) => r.status() === 429);
    expect(tooManyRequests.length).toBeGreaterThan(0);
  });
});
