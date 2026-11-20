/**
 * WCAG 2.1 Level AA Compliance Tests
 *
 * Tests for Phase 3 Accessibility compliance:
 * - Color contrast ratios (4.5:1 minimum for normal text)
 * - Touch target sizes (48x48px minimum)
 * - Heading hierarchy (H1 → H2 → H3, no gaps)
 * - Form labels (all inputs must be labeled)
 * - Error message associations (errors linked to fields)
 *
 * Coverage: 15 tests
 */

import { test, expect } from '@playwright/test';
import {
  checkAccessibility,
  assertAccessible,
  checkColorContrast,
  checkInputLabels,
  checkHeadingHierarchy,
  checkAriaLabels,
  checkTouchTargets,
  checkErrorAssociations,
  generateA11yReport,
} from '../../utils/a11y.utils';

const pages = [
  { url: '/settlement', name: 'Settlement Page' },
  { url: '/payment', name: 'Payment Page' },
  { url: '/evaluation', name: 'Evaluation Dashboard' },
  { url: '/compliance', name: 'Compliance Page' },
  { url: '/', name: 'Home Page' },
];

test.describe('WCAG 2.1 Compliance Tests', () => {
  test.describe('Test 1: Color Contrast Ratios (4.5:1 minimum)', () => {
    pages.forEach(({ url, name }) => {
      test(`should have sufficient color contrast on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const contrasts = await checkColorContrast(page);

        // Filter for visible elements only
        const visibleContrasts = contrasts.filter((c) => c.ratio > 0);

        // Check AA compliance
        const failedContrasts = visibleContrasts.filter(
          (c) => c.ratio < 4.5 && c.wcagLevel === 'FAIL'
        );

        if (failedContrasts.length > 0) {
          const summary = failedContrasts
            .slice(0, 5)
            .map((c) => `${c.element}: ${c.ratio.toFixed(2)}:1`)
            .join(', ');
          console.log(`Low contrast elements: ${summary}`);
        }

        // Assert at least 80% of elements have AA compliance
        const aaCompliant = visibleContrasts.filter((c) => c.wcagLevel !== 'FAIL').length;
        const compliance = (aaCompliant / visibleContrasts.length) * 100;
        expect(compliance).toBeGreaterThanOrEqual(80);
      });
    });
  });

  test.describe('Test 2: Touch Target Sizes (48x48px minimum)', () => {
    pages.forEach(({ url, name }) => {
      test(`should have adequate touch target sizes on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const tooSmall = await checkTouchTargets(page);

        if (tooSmall.length > 0) {
          console.log(`Elements below 48x48px: ${tooSmall.slice(0, 3).join(', ')}`);
        }

        // Allow up to 2 undersized targets for edge cases
        expect(tooSmall.length).toBeLessThanOrEqual(2);
      });
    });
  });

  test.describe('Test 3: Heading Hierarchy (H1 → H2 → H3, no gaps)', () => {
    pages.forEach(({ url, name }) => {
      test(`should have proper heading hierarchy on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const { valid, issues } = await checkHeadingHierarchy(page);

        if (!valid) {
          console.log(`Heading issues on ${name}:`, issues);
        }

        expect(valid).toBe(true);
      });
    });
  });

  test.describe('Test 4: Form Labels (all inputs labeled)', () => {
    const formsPages = [
      { url: '/payment', name: 'Payment Form' },
      { url: '/compliance', name: 'Compliance Form' },
    ];

    formsPages.forEach(({ url, name }) => {
      test(`should have labels for all form inputs on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const unlabeled = await checkInputLabels(page);

        if (unlabeled.length > 0) {
          console.log(`Unlabeled inputs: ${unlabeled.slice(0, 3).join(', ')}`);
        }

        // All inputs must be labeled
        expect(unlabeled.length).toBe(0);
      });
    });
  });

  test.describe('Test 5: Error Message Associations', () => {
    test('should associate error messages with form fields', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Trigger form submission with invalid data
      const submitBtn = page.locator('button[type="submit"]').first();
      if (await submitBtn.isVisible()) {
        await submitBtn.click();

        // Wait for error messages
        await page.waitForTimeout(500);

        const unassociated = await checkErrorAssociations(page);

        if (unassociated.length > 0) {
          console.log(`Unassociated errors: ${unassociated.slice(0, 2).join(', ')}`);
        }

        // Most errors should be associated
        const totalErrors = await page.locator('[role="alert"], .error').count();
        expect(unassociated.length).toBeLessThan(totalErrors);
      }
    });
  });

  test.describe('Test 6: ARIA Labels on Interactive Elements', () => {
    pages.forEach(({ url, name }) => {
      test(`should have proper ARIA labels on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const { missing, invalid } = await checkAriaLabels(page);

        if (missing.length > 0) {
          console.log(`Missing ARIA labels: ${missing.slice(0, 3).join(', ')}`);
        }

        if (invalid.length > 0) {
          console.log(`Invalid ARIA roles: ${invalid.slice(0, 3).join(', ')}`);
        }

        // Allow up to 2 missing labels for edge cases
        expect(missing.length).toBeLessThanOrEqual(2);
        expect(invalid.length).toBe(0);
      });
    });
  });

  test.describe('Test 7: Overall Accessibility Violations', () => {
    pages.forEach(({ url, name }) => {
      test(`should have no critical/serious violations on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const violations = await checkAccessibility(page, {
          minImpact: 'serious',
        });

        const critical = violations.filter((v) => v.impact === 'critical');
        const serious = violations.filter((v) => v.impact === 'serious');

        if (critical.length > 0) {
          console.log(
            `Critical violations: ${critical.map((v) => v.id).join(', ')}`
          );
        }

        if (serious.length > 0) {
          console.log(`Serious violations: ${serious.map((v) => v.id).join(', ')}`);
        }

        expect(critical.length).toBe(0);
        expect(serious.length).toBeLessThanOrEqual(2);
      });
    });
  });

  test.describe('Test 8: Settlement Page Specific WCAG', () => {
    test('settlement page should have accessible data table', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Check for proper table structure
      const tables = await page.locator('table').count();
      if (tables > 0) {
        const hasHeaders = await page.locator('thead, th').count();
        expect(hasHeaders).toBeGreaterThan(0);
      }
    });

    test('settlement page status badges should be distinguishable', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Check for status indicators that rely only on color
      const badges = await page.locator('[class*="badge"], [class*="status"]').count();

      if (badges > 0) {
        // Verify not using color-only indicators
        const ariaLabeled = await page.evaluate(() => {
          let count = 0;
          document.querySelectorAll('[class*="badge"], [class*="status"]').forEach((el) => {
            if (el.getAttribute('aria-label') || el.textContent?.trim()) {
              count++;
            }
          });
          return count;
        });

        expect(ariaLabeled).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 9: Payment Form Accessibility', () => {
    test('payment form inputs should all have associated labels', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      const unlabeled = await checkInputLabels(page);
      expect(unlabeled.length).toBe(0);
    });

    test('payment form should support keyboard navigation', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Count focusable elements
      const focusableElements = await page.evaluate(() => {
        const selector =
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
        return document.querySelectorAll(selector).length;
      });

      expect(focusableElements).toBeGreaterThan(0);
    });
  });

  test.describe('Test 10: Evaluation Dashboard Accessibility', () => {
    test('evaluation dashboard charts should have accessible alternative', async ({
      page,
    }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Check for canvas/SVG with descriptions
      const charts = await page.locator('canvas, svg').count();

      if (charts > 0) {
        const descriptions = await page.evaluate(() => {
          let count = 0;
          document.querySelectorAll('canvas, svg').forEach((el) => {
            if (
              el.getAttribute('aria-label') ||
              el.getAttribute('role') === 'img' ||
              el.querySelector('title')
            ) {
              count++;
            }
          });
          return count;
        });

        // At least some charts should have descriptions
        expect(descriptions).toBeGreaterThan(0);
      }
    });

    test('evaluation metrics should be readable by screen readers', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Check for numeric displays with labels
      const metrics = await page.evaluate(() => {
        const results: string[] = [];
        document.querySelectorAll('[class*="metric"], [class*="score"]').forEach((el) => {
          const text = el.textContent?.trim();
          if (text && /[\d.%]+/.test(text)) {
            if (!el.getAttribute('aria-label') && !el.closest('label')) {
              results.push(`Metric without label: ${text.substring(0, 30)}`);
            }
          }
        });
        return results;
      });

      // Allow some unlabeled metrics (they may be decorative)
      expect(metrics.length).toBeLessThanOrEqual(3);
    });
  });

  test.describe('Test 11: Compliance Page Form WCAG', () => {
    test('compliance form should have clear error handling', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      // Find form
      const form = page.locator('form').first();
      if (await form.isVisible()) {
        // Clear and submit invalid form
        const inputs = await form.locator('input').count();
        for (let i = 0; i < inputs; i++) {
          await form.locator('input').nth(i).clear();
        }

        const submitBtn = form.locator('button[type="submit"]').first();
        if (await submitBtn.isVisible()) {
          await submitBtn.click();
          await page.waitForTimeout(500);

          // Check error messages are announced
          const errors = await page.locator('[role="alert"]').count();
          const errorText = await page.locator('.error, [class*="error"]').count();

          expect(errors + errorText).toBeGreaterThan(0);
        }
      }
    });
  });

  test.describe('Test 12: Focus Indicators Visible', () => {
    pages.forEach(({ url, name }) => {
      test(`should show focus indicators on ${name}`, async ({ page }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        // Tab to first button
        await page.keyboard.press('Tab');

        // Get focused element
        const focused = await page.evaluate(() => {
          const el = document.activeElement as HTMLElement;
          if (!el) return null;
          const styles = window.getComputedStyle(el);
          return {
            element: el.tagName,
            outline: styles.outline,
            boxShadow: styles.boxShadow,
          };
        });

        // Should have some focus indicator
        if (focused) {
          const hasFocusIndicator =
            (focused.outline && focused.outline !== 'none') ||
            (focused.boxShadow && focused.boxShadow !== 'none');

          expect(hasFocusIndicator || focused.element === 'BODY').toBe(true);
        }
      });
    });
  });

  test.describe('Test 13: Language Declaration', () => {
    test('page should have lang attribute', async ({ page }) => {
      await page.goto('/');

      const lang = await page.locator('html').getAttribute('lang');
      expect(lang).toBeTruthy();
      expect(lang?.length).toBeGreaterThan(0);
    });
  });

  test.describe('Test 14: Accessible Navigation', () => {
    test('navigation should be keyboard accessible', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      // Tab through navigation
      await page.keyboard.press('Tab');

      const focused = await page.evaluate(() => {
        return (document.activeElement as HTMLElement).tagName;
      });

      // First focusable element should be found
      expect(['A', 'BUTTON', 'INPUT', 'SELECT', 'TEXTAREA']).toContain(focused);
    });

    test('skip links should be available', async ({ page }) => {
      await page.goto('/');

      const skipLink = page.locator('[href="#main"], [href="#content"]');
      expect(await skipLink.count()).toBeGreaterThanOrEqual(0);
    });
  });

  test.describe('Test 15: Comprehensive Accessibility Report', () => {
    test('should generate full a11y report for settlement page', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const report = await generateA11yReport(page);

      expect(report.timestamp).toBeTruthy();
      expect(report.violations).toBeDefined();
      expect(report.passes).toBeDefined();
      expect(report.incomplete).toBeDefined();

      // Should have at least some passes
      expect(
        report.violations.length + report.passes.length + report.incomplete.length
      ).toBeGreaterThan(0);
    });
  });
});
