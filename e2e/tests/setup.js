// Test setup and global mocks for Genie frontend tests

import { vi } from 'vitest';

// Mock fetch API
global.fetch = vi.fn((url, options) => {
  return Promise.resolve({
    ok: true,
    status: 200,
    statusText: 'OK',
    headers: new Map([['X-CSRF-Token', 'mock-token-' + Date.now()]]),
    text: () => Promise.resolve(''),
    json: () => Promise.resolve({}),
  });
});

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
global.localStorage = localStorageMock;

// Mock sessionStorage
const sessionStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
global.sessionStorage = sessionStorageMock;

// Mock AbortController
global.AbortController = class AbortController {
  constructor() {
    this.signal = { aborted: false };
  }
  abort() {
    this.signal.aborted = true;
  }
};

// Mock TextDecoder
global.TextDecoder = class TextDecoder {
  decode(data, options = {}) {
    if (typeof data === 'string') return data;
    return new TextDecoder().decode(data);
  }
};

// Setup DOM elements
beforeEach(() => {
  document.body.innerHTML = '';
  localStorageMock.getItem.mockClear();
  localStorageMock.setItem.mockClear();
  localStorageMock.removeItem.mockClear();
  localStorageMock.clear.mockClear();
  sessionStorageMock.getItem.mockClear();
  sessionStorageMock.setItem.mockClear();
  sessionStorageMock.removeItem.mockClear();
  sessionStorageMock.clear.mockClear();
  vi.clearAllMocks();
});
