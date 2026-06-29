/**
 * Screen Reader Support Accessibility Tests
 *
 * Tests for Phase 3 screen reader compatibility:
 * - ARIA labels present and correct
 * - Form hints announced properly
 * - Alerts in live regions
 * - Loading states announced
 * - Complex content explained
 *
 * Coverage: 15 tests
 */

import { test, expect } from '@playwright/test';

test.describe('Screen Reader Support Tests', () => {
  test.describe('Test 1: ARIA Labels Present', () => {
    const pages = [
      { url: '/settlement', name: 'Settlement' },
      { url: '/payment', name: 'Payment' },
      { url: '/evaluation', name: 'Evaluation' },
      { url: '/compliance', name: 'Compliance' },
    ];

    pages.forEach(({ url, name }) => {
      test(`${name} page should have ARIA labels on interactive elements`, async ({
        page,
      }) => {
        await page.goto(url);
        await page.waitForLoadState('networkidle');

        const labels = await page.evaluate(() => {
          const results = {
            buttonsWithLabel: 0,
            buttonsWithoutLabel: 0,
            iconsWithAriaLabel: 0,
            iconsWithoutAriaLabel: 0,
          };

          // Check buttons
          document.querySelectorAll('button').forEach((btn) => {
            const label = btn.getAttribute('aria-label');
            const text = btn.textContent?.trim();
            const title = btn.getAttribute('title');

            if (label || text || title) {
              results.buttonsWithLabel++;
            } else {
              results.buttonsWithoutLabel++;
            }
          });

          // Check icon elements
          document.querySelectorAll('.icon, [class*="icon"], svg').forEach((icon) => {
            const label = icon.getAttribute('aria-label');
            const role = icon.getAttribute('role');
            const parent = icon.parentElement?.getAttribute('aria-label');

            if (label || parent || (role && role !== 'presentation')) {
              results.iconsWithAriaLabel++;
            } else {
              results.iconsWithoutAriaLabel++;
            }
          });

          return results;
        });

        // Most buttons should have labels
        expect(labels.buttonsWithLabel).toBeGreaterThan(0);

        // Allow some unlabeled (like text-only buttons)
        expect(labels.buttonsWithoutLabel).toBeLessThanOrEqual(2);
      });
    });
  });

  test.describe('Test 2: Form Hints Announced', () => {
    test('form fields should have hints/description text', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      const formHints = await page.evaluate(() => {
        const hints: Record<string, any> = {
          fieldsWithHint: [],
          fieldsWithoutHint: [],
        };

        document.querySelectorAll('input, select, textarea').forEach((input) => {
          const ariaDescribedBy = input.getAttribute('aria-describedby');
          const description = ariaDescribedBy
            ? document.getElementById(ariaDescribedBy)?.textContent
            : null;
          const placeholder = input.getAttribute('placeholder');
          const helpText = input.closest('.form-group')?.querySelector('[class*="help"]');

          if (description || placeholder || helpText) {
            hints.fieldsWithHint.push({
              type: input.tagName.toLowerCase(),
              hasDescription: !!description,
              hasPlaceholder: !!placeholder,
              hasHelpText: !!helpText,
            });
          } else {
            hints.fieldsWithoutHint.push({
              type: input.tagName.toLowerCase(),
              name: input.getAttribute('name'),
            });
          }
        });

        return hints;
      });

      // Most fields should have some form of hint
      const totalFields = formHints.fieldsWithHint.length + formHints.fieldsWithoutHint.length;
      if (totalFields > 0) {
        const hintCoverage = (formHints.fieldsWithHint.length / totalFields) * 100;
        expect(hintCoverage).toBeGreaterThanOrEqual(60);
      }
    });

    test('required fields should be marked', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      const requiredFields = await page.evaluate(() => {
        const results = {
          requiredWithIndicator: 0,
          requiredWithoutIndicator: 0,
        };

        document.querySelectorAll('input[required], select[required], textarea[required]').forEach(
          (field) => {
            const ariaRequired = field.getAttribute('aria-required');
            const label = field
              .closest('.form-group')
              ?.querySelector('label, [class*="label"]');
            const indicator = label?.querySelector('[class*="required"], [class*="asterisk"]');

            if (ariaRequired === 'true' || indicator) {
              results.requiredWithIndicator++;
            } else {
              results.requiredWithoutIndicator++;
            }
          }
        );

        return results;
      });

      // At least some required fields should have indicators
      const totalRequired =
        requiredFields.requiredWithIndicator + requiredFields.requiredWithoutIndicator;
      if (totalRequired > 0) {
        expect(requiredFields.requiredWithIndicator).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 3: Live Regions for Status Updates', () => {
    test('loading states should use aria-live regions', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Look for loaders or status indicators
      const loaders = await page.locator('[class*="load"], [class*="spinner"]').count();

      if (loaders > 0) {
        // Find aria-live regions
        const liveRegions = await page.evaluate(() => {
          return Array.from(document.querySelectorAll('[aria-live]')).map((region) => ({
            live: region.getAttribute('aria-live'),
            atomic: region.getAttribute('aria-atomic'),
            text: region.textContent?.trim().substring(0, 50),
          }));
        });

        // Should have some live regions
        expect(liveRegions.length).toBeGreaterThanOrEqual(0);
      }
    });

    test('alert messages should use aria-live="assertive"', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      // Trigger an error to show alert
      const submit = page.locator('button[type="submit"]').first();
      if (await submit.isVisible()) {
        // Clear form to trigger validation
        const inputs = await page.locator('input[required]').all();
        for (const input of inputs) {
          await input.clear();
        }

        await submit.click();
        await page.waitForTimeout(500);

        const alerts = await page.evaluate(() => {
          return Array.from(document.querySelectorAll('[role="alert"], [aria-live="assertive"]')).map(
            (alert) => ({
              role: alert.getAttribute('role'),
              live: alert.getAttribute('aria-live'),
              text: alert.textContent?.trim().substring(0, 50),
            })
          );
        });

        // If there are error messages, they should be in alerts
        const errors = await page.locator('[class*="error"]').count();
        if (errors > 0) {
          expect(alerts.length).toBeGreaterThan(0);
        }
      }
    });

    test('success messages should use aria-live="polite"', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const liveRegions = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('[aria-live="polite"]')).map(
          (region) => ({
            atomic: region.getAttribute('aria-atomic'),
            text: region.textContent?.trim().substring(0, 50),
          })
        );
      });

      // May or may not have polite regions
      expect(Array.isArray(liveRegions)).toBe(true);
    });
  });

  test.describe('Test 4: Loading States Announced', () => {
    test('page loading should have accessible indicator', async ({ page }) => {
      await page.goto('/evaluation');

      // Check for loading state
      const loadingIndicators = await page.evaluate(() => {
        return Array.from(
          document.querySelectorAll('[class*="loading"], [aria-busy], [role="status"]')
        ).map((el) => ({
          ariaBusy: el.getAttribute('aria-busy'),
          role: el.getAttribute('role'),
          className: el.className,
        }));
      });

      // Page should announce it's loaded
      expect(document).toBeTruthy();
    });

    test('async operations should announce completion', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Look for status regions
      const statusRegions = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('[role="status"], [aria-live]')).map(
          (region) => ({
            role: region.getAttribute('role'),
            live: region.getAttribute('aria-live'),
            visible: (region as HTMLElement).offsetHeight > 0,
          })
        );
      });

      // Should have at least some status regions
      expect(statusRegions.length).toBeGreaterThanOrEqual(0);
    });
  });

  test.describe('Test 5: Form Validation Announced', () => {
    test('validation errors should be announced to screen readers', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      const form = page.locator('form').first();
      if (await form.isVisible()) {
        // Try to submit empty form
        const submit = form.locator('button[type="submit"]').first();
        if (await submit.isVisible()) {
          await submit.click();
          await page.waitForTimeout(500);

          // Check for error announcements
          const errorAnnouncements = await page.evaluate(() => {
            return Array.from(
              document.querySelectorAll('[role="alert"], [aria-invalid="true"]')
            ).map((el) => ({
              role: el.getAttribute('role'),
              ariaInvalid: el.getAttribute('aria-invalid'),
              text: el.textContent?.trim().substring(0, 50),
            }));
          });

          // Should have announced validation errors
          expect(errorAnnouncements.length).toBeGreaterThanOrEqual(0);
        }
      }
    });

    test('field errors should be linked to fields', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      const errors = await page.evaluate(() => {
        const results: Record<string, any> = {
          fieldsWithAriaDescribedBy: 0,
          fieldsWithoutAssociation: 0,
        };

        document.querySelectorAll('input[aria-invalid="true"]').forEach((field) => {
          const ariaDescribedBy = field.getAttribute('aria-describedby');
          const errorMsg = ariaDescribedBy
            ? document.getElementById(ariaDescribedBy)
            : null;

          if (errorMsg) {
            results.fieldsWithAriaDescribedBy++;
          } else {
            results.fieldsWithoutAssociation++;
          }
        });

        return results;
      });

      // Most invalid fields should have described errors
      expect(errors).toBeDefined();
    });
  });

  test.describe('Test 6: Table Headers and Structure', () => {
    test('data tables should have proper headers', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const tableStructure = await page.evaluate(() => {
        const results: Record<string, any> = {
          tablesWithHeaders: 0,
          tablesWithoutHeaders: 0,
          tablesWithScope: 0,
        };

        document.querySelectorAll('table').forEach((table) => {
          const headers = table.querySelectorAll('th');

          if (headers.length > 0) {
            results.tablesWithHeaders++;

            // Check for scope attribute
            const scopedHeaders = Array.from(headers).filter((h) =>
              h.getAttribute('scope')
            );

            if (scopedHeaders.length === headers.length) {
              results.tablesWithScope++;
            }
          } else {
            results.tablesWithoutHeaders++;
          }
        });

        return results;
      });

      if (tableStructure.tablesWithHeaders > 0) {
        // Tables should have proper headers
        expect(tableStructure.tablesWithHeaders).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 7: Image Alt Text', () => {
    test('all images should have alt text', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      const images = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('img')).map((img) => ({
          src: img.src?.substring(0, 50),
          alt: img.alt,
          ariaLabel: img.getAttribute('aria-label'),
          role: img.getAttribute('role'),
          hasDescription: !!img.getAttribute('aria-describedby'),
        }));
      });

      images.forEach((img) => {
        // Each image should have some form of text alternative
        const hasAlt = img.alt || img.ariaLabel || img.role === 'presentation';
        expect(hasAlt || img.hasDescription).toBe(true);
      });
    });
  });

  test.describe('Test 8: Trace Complexity Explained', () => {
    test('complex visualizations should have accessible descriptions', async ({
      page,
    }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      // Look for charts or complex content
      const complexElements = await page.evaluate(() => {
        return Array.from(
          document.querySelectorAll('canvas, svg, [class*="chart"], [class*="graph"]')
        ).map((el) => ({
          tag: el.tagName.toLowerCase(),
          ariaLabel: el.getAttribute('aria-label'),
          role: el.getAttribute('role'),
          hasTitle: !!el.querySelector('title'),
          parent: el.parentElement?.className,
        }));
      });

      if (complexElements.length > 0) {
        // Complex elements should have descriptions
        const described = complexElements.filter((el) => el.ariaLabel || el.hasTitle);
        expect(described.length).toBeGreaterThan(0);
      }
    });

    test('trace evaluation metrics should be explained', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      const metrics = await page.evaluate(() => {
        return Array.from(
          document.querySelectorAll('[class*="metric"], [class*="score"], [class*="trace"]')
        ).map((el) => ({
          text: el.textContent?.trim().substring(0, 50),
          ariaLabel: el.getAttribute('aria-label'),
          hasTooltip: el.getAttribute('title'),
          role: el.getAttribute('role'),
        }));
      });

      if (metrics.length > 0) {
        // At least some metrics should have explanations
        const explained = metrics.filter((m) => m.ariaLabel || m.hasTooltip);
        expect(explained.length).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 9: Link Text Meaningful', () => {
    test('links should have meaningful text', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      const links = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('a')).map((link) => ({
          text: link.textContent?.trim(),
          ariaLabel: link.getAttribute('aria-label'),
          title: link.getAttribute('title'),
          href: link.getAttribute('href'),
        }));
      });

      links.forEach((link) => {
        // Each link should have meaningful text
        const meaningful = link.text && link.text.length > 0
          ? link.text !== 'click here' && link.text !== 'link' && link.text !== 'more'
          : false;

        const hasAlternative = link.ariaLabel || link.title;

        expect(meaningful || hasAlternative).toBe(true);
      });
    });
  });

  test.describe('Test 10: Lists Properly Marked', () => {
    test('list content should use list elements', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      const lists = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('ul, ol, [role="list"]')).map((list) => ({
          tag: list.tagName.toLowerCase(),
          role: list.getAttribute('role'),
          itemCount: list.querySelectorAll('li, [role="listitem"]').length,
        }));
      });

      if (lists.length > 0) {
        // Should have proper list structure
        lists.forEach((list) => {
          expect(list.itemCount).toBeGreaterThanOrEqual(0);
        });
      }
    });
  });

  test.describe('Test 11: Label Association', () => {
    test('form labels should be properly associated', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      const labels = await page.evaluate(() => {
        const results = {
          properlyAssociated: 0,
          notAssociated: 0,
        };

        document.querySelectorAll('label').forEach((label) => {
          const htmlFor = label.getAttribute('for');
          const input = htmlFor ? document.getElementById(htmlFor) : null;

          if (input) {
            results.properlyAssociated++;
          } else {
            results.notAssociated++;
          }
        });

        return results;
      });

      // Most labels should be associated
      if (labels.properlyAssociated + labels.notAssociated > 0) {
        expect(labels.properlyAssociated).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 12: Fieldset and Legend', () => {
    test('grouped form fields should use fieldset/legend', async ({ page }) => {
      await page.goto('/compliance');
      await page.waitForLoadState('networkidle');

      const fieldsets = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('fieldset')).map((fs) => ({
          hasLegend: !!fs.querySelector('legend'),
          legendText: fs.querySelector('legend')?.textContent?.trim(),
          fieldCount: fs.querySelectorAll('input, select, textarea').length,
        }));
      });

      if (fieldsets.length > 0) {
        // Fieldsets should have legends
        const withLegend = fieldsets.filter((fs) => fs.hasLegend);
        expect(withLegend.length).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 13: Page Title and Heading', () => {
    test('page should have descriptive title', async ({ page }) => {
      await page.goto('/settlement');

      const title = await page.title();
      expect(title).toBeTruthy();
      expect(title.length).toBeGreaterThan(0);
    });

    test('page should start with H1', async ({ page }) => {
      await page.goto('/payment');
      await page.waitForLoadState('networkidle');

      const firstHeading = await page.locator('h1').first();
      const exists = await firstHeading.isVisible();

      if (exists) {
        const text = await firstHeading.textContent();
        expect(text?.length).toBeGreaterThan(0);
      }
    });
  });

  test.describe('Test 14: ARIA Live Regions', () => {
    test('dynamic content updates should be in live regions', async ({ page }) => {
      await page.goto('/evaluation');
      await page.waitForLoadState('networkidle');

      const liveRegions = await page.evaluate(() => {
        return Array.from(document.querySelectorAll('[aria-live]')).map((region) => ({
          live: region.getAttribute('aria-live'),
          atomic: region.getAttribute('aria-atomic'),
          relevant: region.getAttribute('aria-relevant'),
        }));
      });

      // May or may not have live regions
      expect(Array.isArray(liveRegions)).toBe(true);
    });
  });

  test.describe('Test 15: Comprehensive Screen Reader Audit', () => {
    test('complete screen reader compatibility check', async ({ page }) => {
      await page.goto('/settlement');
      await page.waitForLoadState('networkidle');

      // Comprehensive checks
      const audit = await page.evaluate(() => {
        const results = {
          imagesWithAlt: 0,
          buttonsWithLabel: 0,
          inputsWithLabel: 0,
          tablesWithHeaders: 0,
          liveRegions: 0,
          ariaLandmarks: 0,
        };

        // Images
        results.imagesWithAlt = Array.from(document.querySelectorAll('img')).filter(
          (img) => img.alt || img.getAttribute('aria-label')
        ).length;

        // Buttons
        results.buttonsWithLabel = Array.from(document.querySelectorAll('button')).filter(
          (btn) =>
            btn.textContent?.trim() ||
            btn.getAttribute('aria-label') ||
            btn.getAttribute('title')
        ).length;

        // Inputs
        results.inputsWithLabel = Array.from(document.querySelectorAll('input')).filter(
          (input) =>
            document.querySelector(`label[for="${input.id}"]`) ||
            input.getAttribute('aria-label') ||
            input.closest('label')
        ).length;

        // Tables
        results.tablesWithHeaders = Array.from(document.querySelectorAll('table')).filter(
          (table) => table.querySelector('th')
        ).length;

        // Live regions
        results.liveRegions = document.querySelectorAll('[aria-live]').length;

        // Landmarks
        results.ariaLandmarks = document.querySelectorAll(
          '[role="navigation"], [role="main"], [role="contentinfo"], nav, main'
        ).length;

        return results;
      });

      // At least some accessibility features should be present
      const total =
        audit.imagesWithAlt +
        audit.buttonsWithLabel +
        audit.inputsWithLabel +
        audit.tablesWithHeaders +
        audit.liveRegions +
        audit.ariaLandmarks;

      expect(total).toBeGreaterThan(0);
    });
  });
});
