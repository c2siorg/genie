import { test as base, expect } from '@playwright/test';
import * as jwt from 'jsonwebtoken';

/**
 * Authentication Fixture
 * Provides JWT tokens, login helpers, and auth state management
 */

interface AuthFixture {
  // Generate a valid JWT token
  generateJWT: (payload: Record<string, any>, secret?: string) => string;

  // Generate test user token
  getTestUserToken: (userId?: string, roles?: string[]) => string;

  // Generate merchant token
  getMerchantToken: (merchantId?: string) => string;

  // Generate compliance officer token
  getComplianceToken: () => string;

  // Login and set auth context
  login: (email: string, password: string) => Promise<void>;

  // Get current auth headers
  getAuthHeaders: (token?: string) => Record<string, string>;

  // Clear auth state
  logout: () => Promise<void>;

  // Verify CSRF token is present
  verifyCsrfToken: (csrfToken: string) => boolean;
}

export const authFixture = base.extend<AuthFixture>({
  generateJWT: async ({}, use) => {
    const secret = process.env.JWT_SECRET || 'test-secret-key-for-genie';

    const generateJWT = (payload: Record<string, any>, customSecret?: string) => {
      return jwt.sign(payload, customSecret || secret, {
        algorithm: 'HS256',
        expiresIn: '24h',
      });
    };

    await use(generateJWT);
  },

  getTestUserToken: async ({ generateJWT }, use) => {
    const token = (userId = 'test-user-001', roles = ['customer']) => {
      return generateJWT({
        sub: userId,
        email: `${userId}@example.com`,
        roles: roles,
        iat: Math.floor(Date.now() / 1000),
      });
    };

    await use(token);
  },

  getMerchantToken: async ({ generateJWT }, use) => {
    const token = (merchantId = 'merchant-001') => {
      return generateJWT({
        sub: merchantId,
        email: `${merchantId}@merchant.com`,
        roles: ['merchant'],
        merchant_id: merchantId,
      });
    };

    await use(token);
  },

  getComplianceToken: async ({ generateJWT }, use) => {
    const token = () => {
      return generateJWT({
        sub: 'compliance-001',
        email: 'compliance@genie.com',
        roles: ['compliance', 'admin'],
      });
    };

    await use(token);
  },

  login: async ({ page }, use) => {
    const login = async (email: string, password: string) => {
      await page.goto('/login');
      await page.fill('[name="email"]', email);
      await page.fill('[name="password"]', password);
      await page.click('button[type="submit"]');
      await page.waitForURL('/dashboard');
    };

    await use(login);
  },

  getAuthHeaders: async ({}, use) => {
    const headers = (token?: string) => {
      return {
        'Content-Type': 'application/json',
        ...(token && { Authorization: `Bearer ${token}` }),
        'User-Agent': 'Genie-E2E-Test/1.0',
      };
    };

    await use(headers);
  },

  logout: async ({ page }, use) => {
    const logout = async () => {
      await page.goto('/logout');
      await page.waitForURL('/login');
    };

    await use(logout);
  },

  verifyCsrfToken: async ({}, use) => {
    const verify = (token: string) => {
      // CSRF token should be a non-empty string
      return typeof token === 'string' && token.length > 0;
    };

    await use(verify);
  },
});

export { expect };
