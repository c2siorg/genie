import { test as base, APIRequestContext, expect } from '@playwright/test';

/**
 * API Fixture
 * Provides request helpers for API testing with auth, retry logic, and assertions
 */

interface APIFixture {
  // Authenticated API request
  apiRequest: (token?: string) => APIRequestContext;

  // Helper methods
  api: {
    // Create order
    createOrder: (data: any, token?: string) => Promise<any>;

    // Get order details
    getOrder: (orderId: string, token?: string) => Promise<any>;

    // Initiate payment
    initiatePayment: (data: any, token?: string) => Promise<any>;

    // Confirm payment
    confirmPayment: (paymentId: string, token?: string) => Promise<any>;

    // Get settlement status
    getSettlement: (settlementId: string, token?: string) => Promise<any>;

    // Submit compliance check
    submitCompliance: (data: any, token?: string) => Promise<any>;

    // Get compliance status
    getComplianceStatus: (orderId: string, token?: string) => Promise<any>;

    // Submit evaluation feedback
    submitEvalFeedback: (traceId: string, feedback: any, token?: string) => Promise<any>;

    // Get evaluation traces
    getEvalTraces: (token?: string) => Promise<any>;

    // Generic request method
    request: (method: string, endpoint: string, options?: any, token?: string) => Promise<any>;
  };
}

export const apiFixture = base.extend<APIFixture>({
  apiRequest: async ({ request }, use) => {
    const getContext = (token?: string) => {
      const context = {
        async request(method: string, url: string, options?: any) {
          return request[method.toLowerCase() as keyof APIRequestContext](url, {
            ...options,
            headers: {
              ...(options?.headers || {}),
              'Content-Type': 'application/json',
              ...(token && { Authorization: `Bearer ${token}` }),
            },
          });
        },
      };
      return context as APIRequestContext;
    };

    await use(getContext);
  },

  api: async ({ apiRequest }, use) => {
    const baseURL = process.env.BASE_URL || 'http://localhost:8080';
    const ctx = apiRequest();

    const apiHelpers = {
      async createOrder(data: any, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('POST', `${baseURL}/v1/commerce/orders`, {
          data,
        });
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async getOrder(orderId: string, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('GET', `${baseURL}/v1/commerce/orders/${orderId}`);
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async initiatePayment(data: any, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('POST', `${baseURL}/v1/payment/initiate`, {
          data,
        });
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async confirmPayment(paymentId: string, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('POST', `${baseURL}/v1/payment/confirm`, {
          data: { payment_id: paymentId },
        });
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async getSettlement(settlementId: string, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request(
          'GET',
          `${baseURL}/v1/settlement/status/${settlementId}`
        );
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async submitCompliance(data: any, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('POST', `${baseURL}/v1/compliance/check`, {
          data,
        });
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async getComplianceStatus(orderId: string, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request(
          'GET',
          `${baseURL}/v1/compliance/status/${orderId}`
        );
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async submitEvalFeedback(traceId: string, feedback: any, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('POST', `${baseURL}/v1/eval/traces/${traceId}/feedback`, {
          data: feedback,
        });
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async getEvalTraces(token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request('GET', `${baseURL}/v1/eval/traces`);
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },

      async request(method: string, endpoint: string, options?: any, token?: string) {
        const ctx = apiRequest(token);
        const response = await ctx.request(method, `${baseURL}${endpoint}`, options);
        expect(response.status()).toBeLessThan(400);
        return response.json();
      },
    };

    await use(apiHelpers);
  },
});

export { expect };
