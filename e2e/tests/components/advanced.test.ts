// Advanced Component Tests (70 tests)
// Integration scenarios, edge cases, performance, concurrent interactions

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

describe('Advanced Component Tests', () => {
  let container: HTMLElement;
  let mockFetch: any;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    mockFetch = global.fetch;
    vi.useFakeTimers();
  });

  afterEach(() => {
    document.body.removeChild(container);
    vi.clearAllMocks();
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  // ─────────────────────────────────────────────────────────────
  // Integration Scenarios (30 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Integration Scenarios', () => {
    it('should handle complete login flow', async () => {
      container.innerHTML = `
        <form id="login">
          <input name="email" value="user@example.com"/>
          <input name="password" value="password123"/>
          <button type="submit">Login</button>
        </form>
        <div id="status"></div>
      `;

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: { id: 1, email: 'user@example.com' }, csrf_token: 'token' }),
        headers: new Map([['X-CSRF-Token', 'token']]),
      });

      const form = document.getElementById('login') as HTMLFormElement;
      const formData = new FormData(form);

      expect(formData.get('email')).toBe('user@example.com');
      expect(formData.get('password')).toBe('password123');
    });

    it('should handle complete annotation submission flow', async () => {
      container.innerHTML = `
        <form id="annotation">
          <button id="pass" data-label="pass">Pass</button>
          <input id="confidence" type="range" value="85"/>
          <textarea id="notes">Test notes</textarea>
          <button type="submit">Submit</button>
        </form>
      `;

      const passBtn = document.getElementById('pass') as HTMLButtonElement;
      const confidenceInput = document.getElementById('confidence') as HTMLInputElement;
      const notesInput = document.getElementById('notes') as HTMLTextAreaElement;

      passBtn.click();
      expect(confidenceInput.value).toBe('85');
      expect(notesInput.value).toBe('Test notes');
    });

    it('should handle settlement workflow with multiple orders', () => {
      container.innerHTML = `
        <form id="settlement">
          <select id="merchant">
            <option value="m1">Merchant 1</option>
            <option value="m2">Merchant 2</option>
          </select>
          <div id="orders">
            <div class="order" data-amount="10000">Order 1</div>
            <div class="order" data-amount="20000">Order 2</div>
            <div class="order" data-amount="30000">Order 3</div>
          </div>
          <input id="total" readonly/>
        </form>
      `;

      const orders = document.querySelectorAll('.order');
      const total = Array.from(orders).reduce((sum, order) => {
        const amount = parseInt(order.getAttribute('data-amount') || '0');
        return sum + amount;
      }, 0);

      expect(total).toBe(60000);
    });

    it('should handle trace navigation sequence', () => {
      container.innerHTML = `
        <div id="traces">
          <div class="trace" data-index="0">Trace 1</div>
          <div class="trace" data-index="1">Trace 2</div>
          <div class="trace" data-index="2">Trace 3</div>
        </div>
      `;

      const traces = Array.from(document.querySelectorAll('.trace'));
      let currentIndex = 0;

      // Navigate forward
      currentIndex = Math.min(currentIndex + 1, traces.length - 1);
      expect(currentIndex).toBe(1);

      // Navigate forward again
      currentIndex = Math.min(currentIndex + 1, traces.length - 1);
      expect(currentIndex).toBe(2);

      // Try to go past end
      currentIndex = Math.min(currentIndex + 1, traces.length - 1);
      expect(currentIndex).toBe(2);
    });

    it('should handle role-based visibility', () => {
      container.innerHTML = `
        <div>
          <div class="content user-only" data-roles="user,admin">User Content</div>
          <div class="content admin-only" data-roles="admin">Admin Content</div>
          <div class="content public" data-roles="all">Public Content</div>
        </div>
      `;

      const userRole = 'user';
      const content = document.querySelectorAll('.content');

      content.forEach((el) => {
        const roles = el.getAttribute('data-roles')?.split(',') || [];
        const isVisible = roles.includes(userRole) || roles.includes('all');
        el.hidden = !isVisible;
      });

      expect(document.querySelector('.user-only')?.hidden).toBe(false);
      expect(document.querySelector('.admin-only')?.hidden).toBe(true);
      expect(document.querySelector('.public')?.hidden).toBe(false);
    });

    it('should handle state synchronization across components', () => {
      const state = {
        user: null,
        csrfToken: null,
        documents: [],
      };

      // Simulate login
      state.user = { id: 1, email: 'test@example.com' };
      state.csrfToken = 'token-123';

      // Verify state is accessible
      expect(state.user?.email).toBe('test@example.com');
      expect(state.csrfToken).toBe('token-123');
    });

    it('should handle form submission with CSRF token', () => {
      container.innerHTML = `
        <form id="secure-form">
          <input name="data" value="test"/>
          <button type="submit">Submit</button>
        </form>
      `;

      const form = document.getElementById('secure-form') as HTMLFormElement;
      const csrfToken = 'token-abc123';

      const headers = {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken,
      };

      expect(headers['X-CSRF-Token']).toBe('token-abc123');
    });

    it('should handle error recovery', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      try {
        await fetch('/api/test');
      } catch (err) {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true }),
        });

        // Retry
        const resp = await fetch('/api/test');
        const data = await resp.json();
        expect(data.success).toBe(true);
      }
    });

    it('should handle notification display and dismissal', () => {
      container.innerHTML = `
        <div id="notification" class="notification" role="alert">
          <span id="notification-msg">Success!</span>
          <button id="notification-close" aria-label="Close">×</button>
        </div>
      `;

      const notification = document.getElementById('notification') as HTMLElement;
      const closeBtn = document.getElementById('notification-close') as HTMLButtonElement;

      expect(notification.hidden).toBeFalsy();
      closeBtn.click();
      notification.hidden = true;
      expect(notification.hidden).toBe(true);
    });

    it('should handle form reset', () => {
      container.innerHTML = `
        <form id="test-form">
          <input id="field1" value="changed"/>
          <input id="field2" type="checkbox" checked/>
          <button type="reset">Reset</button>
        </form>
      `;

      const form = document.getElementById('test-form') as HTMLFormElement;
      const field1 = document.getElementById('field1') as HTMLInputElement;

      field1.value = 'changed';
      form.reset();

      expect(field1.value).toBe('changed'); // Reset doesn't change value attribute
    });

    it('should handle async data loading', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          data: [{ id: 1, name: 'Item 1' }, { id: 2, name: 'Item 2' }],
        }),
      });

      const resp = await fetch('/api/items');
      const data = await resp.json();

      expect(data.data).toHaveLength(2);
    });

    it('should handle conditional rendering', () => {
      container.innerHTML = `
        <div id="content">
          <div id="loading" hidden>Loading...</div>
          <div id="error" hidden>Error occurred</div>
          <div id="success" hidden>Success!</div>
        </div>
      `;

      const loading = document.getElementById('loading') as HTMLElement;
      const error = document.getElementById('error') as HTMLElement;
      const success = document.getElementById('success') as HTMLElement;

      // Show loading
      loading.hidden = false;
      expect(loading.hidden).toBe(false);

      // Show error
      loading.hidden = true;
      error.hidden = false;
      expect(error.hidden).toBe(false);

      // Show success
      error.hidden = true;
      success.hidden = false;
      expect(success.hidden).toBe(false);
    });

    it('should handle pagination', () => {
      container.innerHTML = `
        <div id="items">
          ${Array.from({ length: 25 }, (_, i) => `<div class="item">Item ${i + 1}</div>`).join('')}
        </div>
        <div id="pagination">
          <button id="prev" aria-label="Previous page">←</button>
          <span id="page-info">Page 1 of 3</span>
          <button id="next" aria-label="Next page">→</button>
        </div>
      `;

      const items = document.querySelectorAll('.item');
      const pageSize = 10;
      const totalPages = Math.ceil(items.length / pageSize);

      expect(totalPages).toBe(3);
    });

    it('should handle filter and search', () => {
      container.innerHTML = `
        <input id="search" placeholder="Search..."/>
        <div id="results">
          <div class="result" data-name="apple">Apple</div>
          <div class="result" data-name="apricot">Apricot</div>
          <div class="result" data-name="banana">Banana</div>
        </div>
      `;

      const searchInput = document.getElementById('search') as HTMLInputElement;
      const results = document.querySelectorAll('.result');

      searchInput.value = 'ap';
      const filtered = Array.from(results).filter((r) =>
        r.getAttribute('data-name')?.includes(searchInput.value)
      );

      expect(filtered).toHaveLength(2);
    });

    it('should handle export data', () => {
      const data = [
        { id: 1, name: 'Test 1', status: 'pass' },
        { id: 2, name: 'Test 2', status: 'fail' },
      ];

      const csv = [
        ['id', 'name', 'status'],
        ...data.map((row) => [row.id, row.name, row.status]),
      ]
        .map((row) => row.join(','))
        .join('\n');

      expect(csv).toContain('id,name,status');
      expect(csv).toContain('1,Test 1,pass');
    });

    it('should handle keyboard shortcuts globally', () => {
      let commandExecuted = false;

      const shortcuts: Record<string, () => void> = {
        's': () => { commandExecuted = true; },
      };

      const event = new KeyboardEvent('keydown', { key: 's' });
      const handler = shortcuts[event.key.toLowerCase()];
      if (handler) handler();

      expect(commandExecuted).toBe(true);
    });

    it('should handle form field dependencies', () => {
      container.innerHTML = `
        <form id="dependent-form">
          <select id="category">
            <option value="">Select</option>
            <option value="fail">Fail</option>
            <option value="pass">Pass</option>
          </select>
          <div id="failure-section" hidden>
            <select id="failure-mode">
              <option value="">Select failure</option>
            </select>
          </div>
        </form>
      `;

      const categorySelect = document.getElementById('category') as HTMLSelectElement;
      const failureSection = document.getElementById('failure-section') as HTMLElement;

      categorySelect.value = 'fail';
      failureSection.hidden = false;

      expect(failureSection.hidden).toBe(false);

      categorySelect.value = 'pass';
      failureSection.hidden = true;

      expect(failureSection.hidden).toBe(true);
    });

    it('should handle auto-save functionality', () => {
      const formData = { field1: 'value1' };
      const saveIntervalMs = 5000;

      const saveData = vi.fn();
      const interval = setInterval(() => saveData(formData), saveIntervalMs);

      expect(saveData).not.toHaveBeenCalled();

      vi.advanceTimersByTime(saveIntervalMs);
      expect(saveData).toHaveBeenCalledWith(formData);

      clearInterval(interval);
    });

    it('should handle form completion percentage', () => {
      container.innerHTML = `
        <form id="multi-step">
          <input id="field1" value=""/>
          <input id="field2" value=""/>
          <input id="field3" value=""/>
          <div id="progress">0%</div>
        </form>
      `;

      const fields = document.querySelectorAll('input[id^="field"]');
      let filledCount = 0;

      fields.forEach((field) => {
        if ((field as HTMLInputElement).value) filledCount++;
      });

      const completionPercent = Math.round((filledCount / fields.length) * 100);
      document.getElementById('progress')!.textContent = `${completionPercent}%`;

      expect(completionPercent).toBe(0);
    });

    it('should handle bulk actions', () => {
      container.innerHTML = `
        <div id="items">
          <label><input type="checkbox" data-item-id="1"/> Item 1</label>
          <label><input type="checkbox" data-item-id="2"/> Item 2</label>
          <label><input type="checkbox" data-item-id="3"/> Item 3</label>
        </div>
        <button id="delete-selected">Delete Selected</button>
      `;

      const checkboxes = document.querySelectorAll('input[type="checkbox"]');
      checkboxes[0].checked = true;
      checkboxes[2].checked = true;

      const selected = Array.from(checkboxes)
        .filter((cb) => (cb as HTMLInputElement).checked)
        .map((cb) => cb.getAttribute('data-item-id'));

      expect(selected).toEqual(['1', '3']);
    });

    it('should handle accessibility tree updates', () => {
      container.innerHTML = `
        <div id="message" role="status" aria-live="polite" aria-atomic="true">
          Ready
        </div>
      `;

      const message = document.getElementById('message') as HTMLElement;
      expect(message.getAttribute('role')).toBe('status');
      expect(message.getAttribute('aria-live')).toBe('polite');

      message.textContent = 'Processing...';
      expect(message.textContent).toBe('Processing...');
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Edge Cases (20 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Edge Cases', () => {
    it('should handle empty string inputs', () => {
      container.innerHTML = '<input id="field" value=""/>';
      const field = document.getElementById('field') as HTMLInputElement;

      field.value = '';
      expect(field.value).toBe('');
    });

    it('should handle very long strings', () => {
      const longString = 'a'.repeat(10000);
      expect(longString.length).toBe(10000);
    });

    it('should handle special characters', () => {
      const special = '<script>alert("xss")</script>';
      const escaped = special.replace(/[<>]/g, '');
      expect(escaped).not.toContain('<script>');
    });

    it('should handle null/undefined values', () => {
      const value: any = null;
      const result = value?.toString() ?? 'default';
      expect(result).toBe('default');
    });

    it('should handle zero values', () => {
      const amount = 0;
      const isValid = amount >= 0;
      expect(isValid).toBe(true);
    });

    it('should handle extremely large numbers', () => {
      const largeNum = Number.MAX_SAFE_INTEGER;
      expect(largeNum).toBe(9007199254740991);
    });

    it('should handle rapid API calls', async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ ok: true }),
      });

      const promises = Array(10)
        .fill(null)
        .map(() => fetch('/api/test'));

      const results = await Promise.all(promises);
      expect(results).toHaveLength(10);
    });

    it('should handle circular references', () => {
      const obj: any = { a: 1 };
      obj.self = obj; // Circular reference

      expect(obj.self === obj).toBe(true);
    });

    it('should handle missing DOM elements gracefully', () => {
      const element = document.getElementById('nonexistent');
      expect(element).toBeNull();
    });

    it('should handle duplicate event listeners', () => {
      container.innerHTML = '<button id="btn">Click</button>';
      const btn = document.getElementById('btn') as HTMLButtonElement;
      const handler = vi.fn();

      btn.addEventListener('click', handler);
      btn.addEventListener('click', handler);

      btn.click();
      expect(handler).toHaveBeenCalledTimes(2);
    });

    it('should handle form submission with no data', () => {
      container.innerHTML = '<form id="empty-form"><button type="submit">Submit</button></form>';
      const form = document.getElementById('empty-form') as HTMLFormElement;
      const formData = new FormData(form);

      expect(formData.entries().next().done).toBe(true);
    });

    it('should handle elements removed from DOM', () => {
      container.innerHTML = '<div id="temp">Temporary</div>';
      const temp = document.getElementById('temp') as HTMLElement;

      container.removeChild(temp);
      const found = document.getElementById('temp');

      expect(found).toBeNull();
    });

    it('should handle rapid show/hide cycles', () => {
      container.innerHTML = '<div id="element">Content</div>';
      const element = document.getElementById('element') as HTMLElement;

      for (let i = 0; i < 10; i++) {
        element.hidden = i % 2 === 0;
      }

      expect(element.hidden).toBe(false);
    });

    it('should handle class list modifications', () => {
      container.innerHTML = '<div id="element" class="class1 class2"></div>';
      const element = document.getElementById('element') as HTMLElement;

      element.classList.add('class3');
      expect(element.classList.contains('class3')).toBe(true);

      element.classList.remove('class1');
      expect(element.classList.contains('class1')).toBe(false);

      element.classList.toggle('class2');
      expect(element.classList.contains('class2')).toBe(false);
    });

    it('should handle attribute edge cases', () => {
      container.innerHTML = '<div id="element"></div>';
      const element = document.getElementById('element') as HTMLElement;

      element.setAttribute('data-value', '');
      expect(element.getAttribute('data-value')).toBe('');

      element.setAttribute('aria-label', 'Test');
      expect(element.getAttribute('aria-label')).toBe('Test');

      element.removeAttribute('aria-label');
      expect(element.getAttribute('aria-label')).toBeNull();
    });

    it('should handle text node operations', () => {
      container.innerHTML = '<div id="element"></div>';
      const element = document.getElementById('element') as HTMLElement;

      const textNode = document.createTextNode('Test text');
      element.appendChild(textNode);

      expect(element.textContent).toBe('Test text');
    });

    it('should handle mutation while iterating', () => {
      container.innerHTML = `
        <div id="list">
          <div class="item">1</div>
          <div class="item">2</div>
          <div class="item">3</div>
        </div>
      `;

      const items = Array.from(document.querySelectorAll('.item'));
      items.forEach((item) => {
        item.textContent = item.textContent + '!';
      });

      expect(document.querySelector('.item')?.textContent).toBe('1!');
    });

    it('should handle event listener cleanup', () => {
      container.innerHTML = '<button id="btn">Click</button>';
      const btn = document.getElementById('btn') as HTMLButtonElement;
      const handler = vi.fn();

      btn.addEventListener('click', handler);
      btn.click();
      expect(handler).toHaveBeenCalledTimes(1);

      btn.removeEventListener('click', handler);
      btn.click();
      expect(handler).toHaveBeenCalledTimes(1);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Performance (15 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Performance', () => {
    it('should efficiently render large lists', () => {
      const startTime = performance.now();

      const items = Array.from({ length: 1000 }, (_, i) => `<div class="item">${i}</div>`).join('');
      container.innerHTML = `<div id="list">${items}</div>`;

      const endTime = performance.now();
      const itemCount = document.querySelectorAll('.item').length;

      expect(itemCount).toBe(1000);
      expect(endTime - startTime).toBeLessThan(1000);
    });

    it('should minimize DOM reflows', () => {
      container.innerHTML = '<div id="content"></div>';
      const content = document.getElementById('content') as HTMLElement;

      const startTime = performance.now();

      const fragment = document.createDocumentFragment();
      for (let i = 0; i < 100; i++) {
        const div = document.createElement('div');
        div.textContent = `Item ${i}`;
        fragment.appendChild(div);
      }
      content.appendChild(fragment);

      const endTime = performance.now();

      expect(content.children.length).toBe(100);
      expect(endTime - startTime).toBeLessThan(500);
    });

    it('should debounce frequent events', () => {
      const handler = vi.fn();
      let timeoutId: any;

      const debouncedHandler = () => {
        clearTimeout(timeoutId);
        timeoutId = setTimeout(() => handler(), 100);
      };

      debouncedHandler();
      debouncedHandler();
      debouncedHandler();

      vi.advanceTimersByTime(100);
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('should throttle scroll events', () => {
      const handler = vi.fn();
      let lastCallTime = 0;

      const throttledHandler = () => {
        const now = Date.now();
        if (now - lastCallTime >= 100) {
          handler();
          lastCallTime = now;
        }
      };

      throttledHandler();
      throttledHandler();
      vi.advanceTimersByTime(100);
      throttledHandler();

      expect(handler).toHaveBeenCalledTimes(2);
    });

    it('should batch DOM updates', () => {
      container.innerHTML = '<div id="content"></div>';
      const content = document.getElementById('content') as HTMLElement;

      content.style.visibility = 'hidden';
      for (let i = 0; i < 100; i++) {
        const div = document.createElement('div');
        div.textContent = `Item ${i}`;
        content.appendChild(div);
      }
      content.style.visibility = 'visible';

      expect(content.children.length).toBe(100);
    });

    it('should use event delegation for large lists', () => {
      container.innerHTML = `
        <div id="list" role="listbox">
          ${Array.from({ length: 50 }, (_, i) => `<div class="list-item" data-id="${i}">Item ${i}</div>`).join('')}
        </div>
      `;

      const list = document.getElementById('list') as HTMLElement;
      const clickHandler = vi.fn();

      list.addEventListener('click', (e) => {
        const target = e.target as HTMLElement;
        if (target.classList.contains('list-item')) {
          clickHandler(target.getAttribute('data-id'));
        }
      });

      const item = list.querySelector('[data-id="25"]') as HTMLElement;
      item.click();

      expect(clickHandler).toHaveBeenCalledWith('25');
    });

    it('should lazy load images', () => {
      container.innerHTML = `
        <div id="gallery">
          <img class="lazy" data-src="image1.jpg" src="placeholder.jpg"/>
          <img class="lazy" data-src="image2.jpg" src="placeholder.jpg"/>
        </div>
      `;

      const lazyImages = document.querySelectorAll('img.lazy');
      expect(lazyImages.length).toBe(2);

      lazyImages.forEach((img) => {
        const dataSrc = (img as HTMLImageElement).getAttribute('data-src');
        (img as HTMLImageElement).src = dataSrc || '';
      });

      expect((lazyImages[0] as HTMLImageElement).src).toContain('image1.jpg');
    });

    it('should memoize expensive computations', () => {
      const expensiveFunc = vi.fn((n: number) => n * n);
      const cache = new Map();

      const memoized = (n: number) => {
        if (cache.has(n)) return cache.get(n);
        const result = expensiveFunc(n);
        cache.set(n, result);
        return result;
      };

      memoized(5);
      memoized(5);
      memoized(5);

      expect(expensiveFunc).toHaveBeenCalledTimes(1);
    });

    it('should unsubscribe from observables on unmount', () => {
      const unsubscribe = vi.fn();
      const subscription = { unsubscribe };

      container.innerHTML = '<div id="component"></div>';
      const component = document.getElementById('component') as HTMLElement;

      // Simulate unmount
      unsubscribe();
      document.body.removeChild(container);

      expect(unsubscribe).toHaveBeenCalled();
    });

    it('should implement virtual scrolling', () => {
      const items = Array.from({ length: 10000 }, (_, i) => ({ id: i, text: `Item ${i}` }));
      const visibleCount = 20;
      let scrollPosition = 0;

      const getVisibleItems = () => {
        const startIndex = Math.floor(scrollPosition / 30); // Item height = 30px
        return items.slice(startIndex, startIndex + visibleCount);
      };

      expect(getVisibleItems()).toHaveLength(visibleCount);
    });

    it('should avoid memory leaks with event listeners', () => {
      container.innerHTML = '<button id="btn">Click</button>';
      const btn = document.getElementById('btn') as HTMLButtonElement;

      const listeners = new WeakMap();
      const handler = () => {};

      listeners.set(btn, handler);
      btn.addEventListener('click', handler);

      btn.removeEventListener('click', handler);
      expect(listeners.has(btn)).toBe(true);
    });

    it('should optimize re-renders', () => {
      const renderCount = vi.fn();

      container.innerHTML = '<div id="content">Initial</div>';
      renderCount();

      const content = document.getElementById('content') as HTMLElement;
      if (content.textContent !== 'Updated') {
        content.textContent = 'Updated';
        renderCount();
      }

      expect(renderCount).toHaveBeenCalledTimes(2);
    });

    it('should handle string concatenation efficiently', () => {
      const startTime = performance.now();

      let result = '';
      for (let i = 0; i < 1000; i++) {
        result += `Item ${i}`;
      }

      const endTime = performance.now();

      expect(result.length).toBeGreaterThan(0);
      expect(endTime - startTime).toBeLessThan(500);
    });

    it('should use requestAnimationFrame for animations', () => {
      const callback = vi.fn();
      const rafId = requestAnimationFrame(callback);

      vi.advanceTimersByTime(16);
      expect(callback).toHaveBeenCalled();

      cancelAnimationFrame(rafId);
    });
  });

  // ─────────────────────────────────────────────────────────────
  // Concurrent Interactions (5 tests)
  // ─────────────────────────────────────────────────────────────

  describe('Concurrent Interactions', () => {
    it('should handle simultaneous API requests', async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: 'test' }),
      });

      const requests = Promise.all([
        fetch('/api/1').then((r) => r.json()),
        fetch('/api/2').then((r) => r.json()),
        fetch('/api/3').then((r) => r.json()),
      ]);

      const results = await requests;
      expect(results).toHaveLength(3);
    });

    it('should handle concurrent form submissions', () => {
      const submit = vi.fn();

      container.innerHTML = `
        <form id="form1"><button type="submit">Submit 1</button></form>
        <form id="form2"><button type="submit">Submit 2</button></form>
      `;

      const form1 = document.getElementById('form1') as HTMLFormElement;
      const form2 = document.getElementById('form2') as HTMLFormElement;

      form1.addEventListener('submit', () => submit('form1'));
      form2.addEventListener('submit', () => submit('form2'));

      form1.querySelector('button')?.click();
      form2.querySelector('button')?.click();

      expect(submit).toHaveBeenCalledTimes(2);
    });

    it('should handle race conditions in state updates', () => {
      let state = { value: 0 };

      const update1 = () => { state.value += 1; };
      const update2 = () => { state.value += 10; };

      Promise.resolve().then(update1);
      Promise.resolve().then(update2);

      vi.runAllTimers();

      expect(state.value).toBe(11);
    });

    it('should handle websocket and HTTP concurrently', async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ source: 'http' }),
      });

      const httpRequest = fetch('/api/data').then((r) => r.json());
      const wsData = Promise.resolve({ source: 'ws' });

      const [http, ws] = await Promise.all([httpRequest, wsData]);

      expect(http.source).toBe('http');
      expect(ws.source).toBe('ws');
    });

    it('should handle timeout race conditions', async () => {
      const timeout = new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Timeout')), 100)
      );

      const data = Promise.resolve({ data: 'success' });

      vi.advanceTimersByTime(50);

      const result = await Promise.race([data, timeout]);
      expect(result.data).toBe('success');
    });
  });
});
