/**
 * Visual Regression Tests for Pages
 *
 * Tests for Phase 3 visual regression testing:
 * - Settlement page (5 states)
 * - Payment page (5 states)
 * - Evaluation dashboard (5 states)
 * - Compliance page (5 states)
 * - Overall layouts (5 states)
 *
 * Uses Playwright screenshot comparison with baselines
 * Coverage: 30 tests
 */

import { test, expect } from '@playwright/test';

test.describe('Visual Regression Tests - Pages', () => {
  test.describe('Settlement Page Visual Regression', () => {
    test('should render settlement page default state', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('settlement-default.png');
    });

    test('should render settlement page with filters applied', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Apply filter
      const filterBtn = page.locator('button:has-text("Filter")').first();
      if (await filterBtn.isVisible()) {
        await filterBtn.click();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('settlement-filtered.png');
    });

    test('should render settlement page with expanded row', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Try to expand a settlement row
      const expandBtn = page.locator('[aria-label*="expand"], [class*="expand"]').first();
      if (await expandBtn.isVisible()) {
        await expandBtn.click();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('settlement-expanded.png');
    });

    test('should render settlement page with modal open', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Look for action button that opens modal
      const actionBtn = page.locator('button[class*="action"]').first();
      if (await actionBtn.isVisible()) {
        await actionBtn.click();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('settlement-modal.png');
    });

    test('should render settlement page with loading state', async ({ page }) => {
      // Intercept and delay network
      await page.route('**/api/**', (route) => {
        setTimeout(() => route.continue(), 1000);
      });

      await page.goto('/settlement');
      // Capture before load completes
      await page.waitForTimeout(200);

      expect(await page.screenshot()).toMatchSnapshot('settlement-loading.png');
    });
  });

  test.describe('Payment Page Visual Regression', () => {
    test('should render payment page default state', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('payment-default.png');
    });

    test('should render payment form with focus states', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Focus first input
      const input = page.locator('input').first();
      if (await input.isVisible()) {
        await input.focus();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('payment-focused.png');
    });

    test('should render payment page with validation errors', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Try to submit empty form
      const submitBtn = page.locator('button[type="submit"]').first();
      if (await submitBtn.isVisible()) {
        await submitBtn.click();
        await page.waitForTimeout(500);
      }

      expect(await page.screenshot()).toMatchSnapshot('payment-errors.png');
    });

    test('should render payment page with populated form', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Fill form
      const inputs = await page.locator('input[type="text"], input[type="email"], input[type="number"]').all();
      for (let i = 0; i < inputs.length && i < 3; i++) {
        await inputs[i].fill(`test value ${i + 1}`);
      }

      await page.waitForTimeout(300);
      expect(await page.screenshot()).toMatchSnapshot('payment-populated.png');
    });

    test('should render payment page with success message', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Simulate success state
      const successMsg = page.locator('[class*="success"], [role="alert"]').first();
      if (await successMsg.isVisible()) {
        // Already showing success
      } else {
        // Try to trigger success
        const submitBtn = page.locator('button[type="submit"]').first();
        if (await submitBtn.isVisible()) {
          // Fill required fields first
          const inputs = await page.locator('input[required]').all();
          for (const input of inputs) {
            await input.fill('test');
          }

          await submitBtn.click();
          await page.waitForTimeout(500);
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('payment-success.png');
    });
  });

  test.describe('Evaluation Dashboard Visual Regression', () => {
    test('should render evaluation dashboard default state', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('evaluation-default.png');
    });

    test('should render evaluation with chart interactions', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Try to interact with chart
      const chart = page.locator('canvas, svg').first();
      if (await chart.isVisible()) {
        const box = await chart.boundingBox();
        if (box) {
          await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
          await page.waitForTimeout(300);
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('evaluation-interactive.png');
    });

    test('should render evaluation with expanded metrics', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Click to expand metric
      const expandBtn = page.locator('[aria-expanded="false"]').first();
      if (await expandBtn.isVisible()) {
        await expandBtn.click();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('evaluation-expanded.png');
    });

    test('should render evaluation with filters applied', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Apply filter
      const filterSelect = page.locator('select').first();
      if (await filterSelect.isVisible()) {
        await filterSelect.selectOption({ index: 1 });
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('evaluation-filtered.png');
    });

    test('should render evaluation with details panel open', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Click a row to show details
      const row = page.locator('tr, [role="row"]').nth(1);
      if (await row.isVisible()) {
        await row.click();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('evaluation-details.png');
    });
  });

  test.describe('Compliance Page Visual Regression', () => {
    test('should render compliance page default state', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('compliance-default.png');
    });

    test('should render compliance form with all fields visible', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      // Scroll to see all fields
      await page.evaluate(() => {
        window.scrollBy(0, window.innerHeight);
      });

      await page.waitForTimeout(300);
      expect(await page.screenshot()).toMatchSnapshot('compliance-scrolled.png');
    });

    test('should render compliance with validation states', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      // Mark some fields as invalid
      const inputs = await page.locator('input').all();
      if (inputs.length > 0) {
        await inputs[0].fill('invalid');
        await inputs[0].evaluate((el) => {
          (el as HTMLInputElement).setCustomValidity('Invalid input');
        });

        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('compliance-invalid.png');
    });

    test('should render compliance with status badge', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      // Look for status indicators
      const badge = page.locator('[class*="badge"], [class*="status"]').first();
      if (await badge.isVisible()) {
        await badge.scrollIntoViewIfNeeded();
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('compliance-status.png');
    });

    test('should render compliance with completion indicator', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      // Look for progress bar
      const progress = page.locator('[role="progressbar"], [class*="progress"]').first();
      if (await progress.isVisible()) {
        await page.waitForTimeout(300);
      }

      expect(await page.screenshot()).toMatchSnapshot('compliance-progress.png');
    });
  });

  test.describe('Overall Layout Visual Regression', () => {
    test('should render header navigation consistently', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      // Focus on header
      const header = page.locator('header, [role="banner"]').first();
      if (await header.isVisible()) {
        const box = await header.boundingBox();
        await page.screenshot({ clip: box || undefined });
      }

      expect(await page.screenshot()).toMatchSnapshot('layout-header.png');
    });

    test('should render sidebar/navigation correctly', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Focus on sidebar
      const sidebar = page.locator('nav, [role="navigation"], aside').first();
      if (await sidebar.isVisible()) {
        const box = await sidebar.boundingBox();
        await page.screenshot({ clip: box || undefined });
      }

      expect(await page.screenshot()).toMatchSnapshot('layout-sidebar.png');
    });

    test('should render footer consistently', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      // Scroll to footer
      await page.evaluate(() => {
        window.scrollTo(0, document.body.scrollHeight);
      });

      await page.waitForTimeout(300);

      const footer = page.locator('footer, [role="contentinfo"]').first();
      if (await footer.isVisible()) {
        expect(await page.screenshot()).toMatchSnapshot('layout-footer.png');
      }
    });

    test('should render main content area layout', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Focus on main content
      const main = page.locator('main, [role="main"]').first();
      if (await main.isVisible()) {
        const box = await main.boundingBox();
        if (box) {
          expect(await page.screenshot({ clip: box })).toMatchSnapshot('layout-main.png');
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('layout-full.png');
    });

    test('should maintain grid/flexbox layout structure', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Check layout consistency
      const gridContainer = page.locator('[class*="grid"], [class*="flex"]').first();
      if (await gridContainer.isVisible()) {
        const box = await gridContainer.boundingBox();
        if (box) {
          expect(await page.screenshot({ clip: box })).toMatchSnapshot(
            'layout-grid.png'
          );
        }
      }

      expect(await page.screenshot()).toMatchSnapshot('layout-container.png');
    });
  });

  test.describe('Responsive Layout Visual Regression', () => {
    test('should render desktop layout with whitespace', async ({ page }) => {
      await page.setViewportSize({ width: 1920, height: 1080 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot(
        'responsive-desktop-full.png'
      );
    });

    test('should render medium viewport layout', async ({ page }) => {
      await page.setViewportSize({ width: 1024, height: 768 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot(
        'responsive-medium.png'
      );
    });

    test('should render small viewport with adjusted spacing', async ({ page }) => {
      await page.setViewportSize({ width: 768, height: 1024 });
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot(
        'responsive-tablet.png'
      );
    });

    test('should maintain alignment at different zoom levels', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Set zoom to 125%
      await page.evaluate(() => {
        document.body.style.zoom = '1.25';
      });

      await page.waitForTimeout(300);
      expect(await page.screenshot()).toMatchSnapshot('responsive-zoom-125.png');
    });

    test('should maintain readability at maximum zoom', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Set zoom to 200%
      await page.evaluate(() => {
        document.body.style.zoom = '2';
      });

      await page.waitForTimeout(300);
      expect(await page.screenshot()).toMatchSnapshot('responsive-zoom-200.png');
    });
  });

  test.describe('Component Visual Consistency', () => {
    test('buttons should have consistent styling', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Find button with different states
      const buttons = await page.locator('button').all();

      for (let i = 0; i < Math.min(buttons.length, 3); i++) {
        const box = await buttons[i].boundingBox();
        if (box) {
          expect(await page.screenshot({ clip: box })).toMatchSnapshot(
            `component-button-${i}.png`
          );
        }
      }
    });

    test('form inputs should have consistent styling', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Find inputs
      const inputs = await page.locator('input[type="text"], input[type="email"]').all();

      for (let i = 0; i < Math.min(inputs.length, 2); i++) {
        const box = await inputs[i].boundingBox();
        if (box) {
          expect(await page.screenshot({ clip: box })).toMatchSnapshot(
            `component-input-${i}.png`
          );
        }
      }
    });

    test('tables should render consistently', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const table = page.locator('table').first();
      if (await table.isVisible()) {
        const box = await table.boundingBox();
        if (box) {
          expect(await page.screenshot({ clip: box })).toMatchSnapshot(
            'component-table.png'
          );
        }
      }
    });
  });

  test.describe('Dark Mode Visual Regression (if supported)', () => {
    test('should render correctly in light mode', async ({ page }) => {
      // Default light mode
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      expect(await page.screenshot()).toMatchSnapshot('darkmode-light.png');
    });

    test('should render correctly in dark mode (if available)', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Try to toggle dark mode
      const themeToggle = page.locator('[aria-label*="theme"], [class*="theme"]').first();
      if (await themeToggle.isVisible()) {
        await themeToggle.click();
        await page.waitForTimeout(300);

        expect(await page.screenshot()).toMatchSnapshot('darkmode-dark.png');
      }
    });
  });
});
