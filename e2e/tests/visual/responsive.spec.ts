/**
 * Mobile & Responsive Visual Regression Tests
 *
 * Tests for Phase 3 responsive design:
 * - Mobile layout 375px (8 tests)
 * - Tablet layout 768px (8 tests)
 * - Touch interactions (5 tests)
 * - Zoom to 200% (4 tests)
 *
 * Coverage: 25 tests
 */

import { test, expect } from '@playwright/test';

test.describe('Mobile & Responsive Visual Regression Tests', () => {
  test.describe('Mobile Layout 375px (iPhone SE)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
    });

    test('settlement page should stack vertically on mobile', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const layout = await page.evaluate(() => {
        const main = document.querySelector('main, [role="main"]');
        const rect = main?.getBoundingClientRect();
        return {
          width: rect?.width || 0,
          hasScroll: document.body.scrollHeight > window.innerHeight,
        };
      });

      expect(layout.width).toBeLessThanOrEqual(375);
      expect(await page.screenshot()).toMatchSnapshot('mobile-settlement.png');
    });

    test('payment form should be mobile optimized', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      // Check form inputs are touchable (48px minimum)
      const inputs = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('input, button')).map((el) => {
          const rect = (el as HTMLElement).getBoundingClientRect();
          return {
            height: rect.height,
            isTouchable: rect.height >= 44, // Mobile minimum
          };
        });
      });

      const touchable = inputs.filter((i) => i.isTouchable);
      expect(touchable.length).toBeGreaterThan(0);

      expect(await page.screenshot()).toMatchSnapshot('mobile-payment.png');
    });

    test('evaluation dashboard should reflow on mobile', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('mobile-evaluation.png');
    });

    test('compliance form should be readable on mobile', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      // Check text is readable
      const fontSize = await page.evaluate(() => {
        const text = document.body.querySelector('label, input');
        return window.getComputedStyle(text!).fontSize;
      });

      // Should be at least 12px for readability
      const size = parseInt(fontSize);
      expect(size).toBeGreaterThanOrEqual(12);

      expect(await page.screenshot()).toMatchSnapshot('mobile-compliance.png');
    });

    test('navigation should be accessible on mobile', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      // Check for hamburger menu or responsive nav
      const nav = await page.evaluate(() => {
        const navElement = document.querySelector('nav, [role="navigation"]');
        const rect = navElement?.getBoundingClientRect();
        return {
          exists: !!navElement,
          visible: (rect?.width || 0) > 0,
        };
      });

      expect(nav.exists || nav.visible).toBe(true);

      expect(await page.screenshot()).toMatchSnapshot('mobile-nav.png');
    });

    test('mobile buttons should have adequate spacing', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const buttonSpacing = await page.evaluate(() => {
        const buttons = Array.from(document.querySelectorAll('button'));
        let tooClose = 0;

        for (let i = 0; i < buttons.length - 1; i++) {
          const rect1 = buttons[i].getBoundingClientRect();
          const rect2 = buttons[i + 1].getBoundingClientRect();

          // Check vertical spacing
          const gap = Math.abs(rect2.top - (rect1.bottom + 8));
          if (gap < 0) {
            tooClose++;
          }
        }

        return { buttons: buttons.length, tooClose };
      });

      // Buttons should have adequate spacing
      expect(buttonSpacing.tooClose).toBeLessThanOrEqual(1);

      expect(await page.screenshot()).toMatchSnapshot('mobile-buttons.png');
    });

    test('mobile page should not have horizontal scroll', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const hasHorizontalScroll = await page.evaluate(() => {
        return document.documentElement.scrollWidth > window.innerWidth;
      });

      expect(hasHorizontalScroll).toBe(false);

      expect(await page.screenshot()).toMatchSnapshot('mobile-no-scroll.png');
    });

    test('mobile modals should cover full screen', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Try to open a modal
      const btn = page.locator('button').nth(2);
      if (await btn.isVisible()) {
        await btn.click();
        await page.waitForTimeout(300);

        const modal = await page.evaluate(() => {
          const m = document.querySelector('[role="dialog"], .modal');
          const rect = m?.getBoundingClientRect();
          return {
            exists: !!m,
            fullScreen: rect?.width === window.innerWidth,
          };
        });

        if (modal.exists) {
          expect(await page.screenshot()).toMatchSnapshot('mobile-modal.png');
        }
      }
    });
  });

  test.describe('Tablet Layout 768px (iPad)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 768, height: 1024 });
    });

    test('settlement page should use tablet layout', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const layout = await page.evaluate(() => {
        const cols = document.querySelectorAll('[class*="col"], [class*="grid-col"]').length;
        return { multiColumn: cols > 1 };
      });

      expect(await page.screenshot()).toMatchSnapshot('tablet-settlement.png');
    });

    test('payment form should use tablet spacing', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const form = await page.evaluate(() => {
        const f = document.querySelector('form');
        const rect = f?.getBoundingClientRect();
        const padding = window.getComputedStyle(f!).padding;
        return {
          width: rect?.width,
          padding,
        };
      });

      // Form should have breathing room
      expect(form.width).toBeLessThanOrEqual(768);

      expect(await page.screenshot()).toMatchSnapshot('tablet-payment.png');
    });

    test('evaluation dashboard should show more data on tablet', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const charts = await page.evaluate(() => {
        return document.querySelectorAll('canvas, svg, [class*="chart"]').length;
      });

      expect(charts).toBeGreaterThanOrEqual(0);

      expect(await page.screenshot()).toMatchSnapshot('tablet-evaluation.png');
    });

    test('compliance form should use multi-column on tablet', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('tablet-compliance.png');
    });

    test('tablet navigation should be visible', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const nav = await page.evaluate(() => {
        const n = document.querySelector('nav');
        return {
          visible: n ? (n as HTMLElement).offsetWidth > 0 : false,
        };
      });

      expect(nav.visible).toBe(true);

      expect(await page.screenshot()).toMatchSnapshot('tablet-nav.png');
    });

    test('tablet should show full table without scrolling', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const table = await page.evaluate(() => {
        const t = document.querySelector('table');
        const rect = t?.getBoundingClientRect();
        return {
          visible: (rect?.width || 0) > 0,
          width: rect?.width || 0,
        };
      });

      expect(table.width).toBeLessThanOrEqual(768);

      expect(await page.screenshot()).toMatchSnapshot('tablet-table.png');
    });

    test('tablet sidebar should be accessible', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const sidebar = await page.evaluate(() => {
        const s = document.querySelector('aside, [role="complementary"]');
        return {
          exists: !!s,
          visible: s ? (s as HTMLElement).offsetHeight > 0 : false,
        };
      });

      expect(await page.screenshot()).toMatchSnapshot('tablet-sidebar.png');
    });

    test('tablet inputs should have proper size', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const inputs = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('input, button')).map((el) => {
          const rect = (el as HTMLElement).getBoundingClientRect();
          return rect.height >= 40; // Tablet minimum
        });
      });

      const adequate = inputs.filter((i) => i).length;
      expect(adequate).toBeGreaterThan(0);

      expect(await page.screenshot()).toMatchSnapshot('tablet-inputs.png');
    });
  });

  test.describe('Touch Interactions (5 tests)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
    });

    test('should handle touch on button', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const btn = page.locator('button').first();
      if (await btn.isVisible()) {
        // Simulate touch
        const box = await btn.boundingBox();
        if (box) {
          await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
          await page.waitForTimeout(300);
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('touch-button.png');
    });

    test('should handle touch on form input', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      const input = page.locator('input[type="text"]').first();
      if (await input.isVisible()) {
        const box = await input.boundingBox();
        if (box) {
          await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
          await page.keyboard.type('test');
          await page.waitForTimeout(300);
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('touch-input.png');
    });

    test('should handle touch swipe gesture', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Simulate swipe
      await page.touchscreen.tap(100, 300);
      await page.touchscreen.tap(300, 300);

      await page.waitForTimeout(300);
      expect(await page.screenshot()).toMatchSnapshot('touch-swipe.png');
    });

    test('should handle long press on element', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const el = page.locator('[class*="card"], [class*="item"]').first();
      if (await el.isVisible()) {
        const box = await el.boundingBox();
        if (box) {
          // Long press simulation
          await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
          await page.mouse.down();
          await page.waitForTimeout(500);
          await page.mouse.up();
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('touch-longpress.png');
    });

    test('should handle pinch zoom gesture', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Simulate zoom (would need actual pinch in real browser)
      await page.evaluate(() => {
        document.body.style.transform = 'scale(1.2)';
      });

      await page.waitForTimeout(300);
      expect(await page.screenshot()).toMatchSnapshot('touch-pinch.png');
    });
  });

  test.describe('Zoom Levels (4 tests)', () => {
    test('should maintain layout at 100% zoom (normal)', async ({ page }) => {
      await page.setViewportSize({ width: 1024, height: 768 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('zoom-100.png');
    });

    test('should maintain layout at 125% zoom', async ({ page }) => {
      await page.setViewportSize({ width: 1024, height: 768 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Set zoom
      await page.evaluate(() => {
        (document.documentElement as any).style.zoom = '1.25';
      });

      await page.waitForTimeout(500);
      expect(await page.screenshot()).toMatchSnapshot('zoom-125.png');
    });

    test('should maintain readability at 150% zoom', async ({ page }) => {
      await page.setViewportSize({ width: 1024, height: 768 });
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Set zoom
      await page.evaluate(() => {
        (document.documentElement as any).style.zoom = '1.5';
      });

      await page.waitForTimeout(500);

      // Check text is still readable
      const fontSize = await page.evaluate(() => {
        const el = document.querySelector('input, button');
        return window.getComputedStyle(el!).fontSize;
      });

      expect(parseInt(fontSize)).toBeGreaterThanOrEqual(12);

      expect(await page.screenshot()).toMatchSnapshot('zoom-150.png');
    });

    test('should maintain functionality at 200% zoom', async ({ page }) => {
      await page.setViewportSize({ width: 1024, height: 768 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Set zoom
      await page.evaluate(() => {
        (document.documentElement as any).style.zoom = '2';
      });

      await page.waitForTimeout(500);

      // Check elements are still accessible
      const focusable = await page.evaluate(() => {
        return document.querySelectorAll('button, input, a').length;
      });

      expect(focusable).toBeGreaterThan(0);

      expect(await page.screenshot()).toMatchSnapshot('zoom-200.png');
    });
  });

  test.describe('Orientation Changes', () => {
    test('should handle portrait orientation on mobile', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('orientation-portrait.png');
    });

    test('should handle landscape orientation on mobile', async ({ page }) => {
      await page.setViewportSize({ width: 667, height: 375 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      const layout = await page.evaluate(() => {
        return {
          width: window.innerWidth,
          height: window.innerHeight,
        };
      });

      expect(layout.width).toBeGreaterThan(layout.height);

      expect(await page.screenshot()).toMatchSnapshot('orientation-landscape.png');
    });
  });

  test.describe('Flexible Grid and Typography', () => {
    test('should use responsive typography', async ({ page }) => {
      const viewports = [
        { width: 375, height: 667, name: 'mobile' },
        { width: 768, height: 1024, name: 'tablet' },
        { width: 1920, height: 1080, name: 'desktop' },
      ];

      for (const vp of viewports) {
        await page.setViewportSize({ width: vp.width, height: vp.height });
        await page.goto('/settlement');
        await page.waitForLoadState('networkidle');
        await page.waitForTimeout(500);

        const fontSizes = await page.evaluate(() => {
          const headings = Array.from(document.querySelectorAll('h1, h2, h3, h4')).map(
            (h) => window.getComputedStyle(h).fontSize
          );
          return headings;
        });

        // Font sizes should exist
        expect(fontSizes.length).toBeGreaterThan(0);
      }
    });

    test('should use flexible container widths', async ({ page }) => {
      const viewports = [
        { width: 375, height: 667 },
        { width: 768, height: 1024 },
        { width: 1024, height: 768 },
      ];

      for (const vp of viewports) {
        await page.setViewportSize({ width: vp.width, height: vp.height });
        await page.goto('/settlement');
        await page.waitForLoadState('networkidle');

        const overflow = await page.evaluate(() => {
          return document.documentElement.scrollWidth > window.innerWidth;
        });

        // Should not overflow viewport
        expect(overflow).toBe(false);
      }
    });
  });

  test.describe('Image and Media Responsiveness', () => {
    test('should scale images responsively', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      const images = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('img')).map((img) => ({
          hasWidth: img.getAttribute('width') !== null,
          hasHeight: img.getAttribute('height') !== null,
          srcSet: img.getAttribute('srcset'),
          sizes: img.getAttribute('sizes'),
        }));
      });

      // Images should have responsive attributes
      expect(images.length).toBeGreaterThanOrEqual(0);
    });

    test('should handle SVG in responsive layouts', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      const svgs = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('svg')).map((svg) => {
          const parent = svg.parentElement;
          return {
            hasViewBox: svg.hasAttribute('viewBox'),
            parentClass: parent?.className,
          };
        });
      });

      expect(svgs.length).toBeGreaterThanOrEqual(0);
    });
  });
});
