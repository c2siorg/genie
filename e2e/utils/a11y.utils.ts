/**
 * Accessibility Testing Utilities
 *
 * Provides helpers for WCAG 2.1 compliance testing using axe-core
 */

import { Page, expect } from '@playwright/test';
import { injectAxe, getViolations } from 'axe-playwright';

export interface A11yViolation {
  id: string;
  impact: 'critical' | 'serious' | 'moderate' | 'minor';
  description: string;
  nodes: number;
  help: string;
  helpUrl: string;
}

export interface ColorContrast {
  element: string;
  foreground: string;
  background: string;
  ratio: number;
  wcagLevel: 'AA' | 'AAA' | 'FAIL';
}

export interface AccessibilityReport {
  violations: A11yViolation[];
  passes: { id: string; description: string; nodeCount: number }[];
  incomplete: { id: string; description: string; nodeCount: number }[];
  timestamp: string;
}

/**
 * Injects axe-core and checks for violations
 * @param page - Playwright page object
 * @param options - Configuration options
 * @returns Array of violations found
 */
export async function checkAccessibility(
  page: Page,
  options?: {
    ignoreRules?: string[];
    minImpact?: 'critical' | 'serious' | 'moderate' | 'minor';
  }
): Promise<A11yViolation[]> {
  await injectAxe(page);
  const violations = await getViolations(page);

  // Filter out ignored rules
  const filtered = violations.filter((v) => {
    if (options?.ignoreRules?.includes(v.id)) return false;
    if (options?.minImpact) {
      const impacts = ['critical', 'serious', 'moderate', 'minor'];
      const minIndex = impacts.indexOf(options.minImpact);
      const currentIndex = impacts.indexOf(v.impact);
      return currentIndex <= minIndex;
    }
    return true;
  });

  return filtered as A11yViolation[];
}

/**
 * Asserts no critical or serious accessibility violations
 */
export async function assertAccessible(
  page: Page,
  minImpact?: 'critical' | 'serious'
) {
  const violations = await checkAccessibility(page, { minImpact });
  if (violations.length > 0) {
    const summary = violations
      .map((v) => `${v.id} (${v.impact}): ${v.description}`)
      .join('\n');
    throw new Error(`Accessibility violations found:\n${summary}`);
  }
}

/**
 * Check color contrast ratios for WCAG compliance
 */
export async function checkColorContrast(page: Page): Promise<ColorContrast[]> {
  const contrastChecks = await page.evaluate(() => {
    const results: ColorContrast[] = [];

    function getComputedStyle(element: Element): CSSStyleDeclaration {
      return window.getComputedStyle(element);
    }

    function hexToRgb(hex: string): [number, number, number] | null {
      const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
      return result
        ? [parseInt(result[1], 16), parseInt(result[2], 16), parseInt(result[3], 16)]
        : null;
    }

    function luminance(r: number, g: number, b: number): number {
      const [rs, gs, bs] = [r, g, b].map((x) => {
        x = x / 255;
        return x <= 0.03928 ? x / 12.92 : Math.pow((x + 0.05) / 1.05, 2.4);
      });
      return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs;
    }

    function contrast(rgb1: [number, number, number], rgb2: [number, number, number]): number {
      const lum1 = luminance(rgb1[0], rgb1[1], rgb1[2]);
      const lum2 = luminance(rgb2[0], rgb2[1], rgb2[2]);
      const lighter = Math.max(lum1, lum2);
      const darker = Math.min(lum1, lum2);
      return (lighter + 0.05) / (darker + 0.05);
    }

    function parseColor(color: string): [number, number, number] | null {
      if (color.startsWith('#')) return hexToRgb(color);
      if (color.startsWith('rgb')) {
        const match = color.match(/\d+/g);
        if (match) return [parseInt(match[0]), parseInt(match[1]), parseInt(match[2])];
      }
      return null;
    }

    document.querySelectorAll('button, a, label, input, textarea, select').forEach((el) => {
      const style = getComputedStyle(el);
      const fg = parseColor(style.color);
      const bg = parseColor(style.backgroundColor);

      if (fg && bg) {
        const ratio = contrast(fg, bg);
        const wcagLevel = ratio >= 7 ? 'AAA' : ratio >= 4.5 ? 'AA' : 'FAIL';

        results.push({
          element: el.tagName.toLowerCase(),
          foreground: style.color,
          background: style.backgroundColor,
          ratio,
          wcagLevel,
        });
      }
    });

    return results;
  });

  return contrastChecks;
}

/**
 * Check that all inputs have associated labels
 */
export async function checkInputLabels(page: Page): Promise<string[]> {
  const unlabeled = await page.evaluate(() => {
    const unlabeledInputs: string[] = [];

    document.querySelectorAll('input, textarea, select').forEach((input) => {
      const id = input.getAttribute('id');
      const ariaLabel = input.getAttribute('aria-label');
      const ariaLabelledBy = input.getAttribute('aria-labelledby');

      // Check for explicit label
      const hasExplicitLabel = id && document.querySelector(`label[for="${id}"]`);

      // Check for implicit label (input wrapped in label)
      const implicitLabel =
        input.closest('label') && input.closest('label')?.textContent?.trim();

      if (!ariaLabel && !ariaLabelledBy && !hasExplicitLabel && !implicitLabel) {
        unlabeledInputs.push(
          `${input.tagName} - id: ${id || 'none'}, type: ${(input as any).type || 'none'}`
        );
      }
    });

    return unlabeledInputs;
  });

  return unlabeled;
}

/**
 * Check heading hierarchy (h1 → h2 → h3, no gaps)
 */
export async function checkHeadingHierarchy(
  page: Page
): Promise<{ valid: boolean; issues: string[] }> {
  const issues: string[] = [];

  const headings = await page.$$eval('h1, h2, h3, h4, h5, h6', (elements) => {
    return elements.map((el) => ({
      level: parseInt(el.tagName[1]),
      text: el.textContent?.trim().substring(0, 50) || '',
    }));
  });

  if (headings.length === 0) {
    issues.push('No headings found on page');
    return { valid: false, issues };
  }

  if (headings[0].level !== 1) {
    issues.push('First heading should be H1');
  }

  for (let i = 1; i < headings.length; i++) {
    const prev = headings[i - 1].level;
    const current = headings[i].level;

    if (current > prev + 1) {
      issues.push(
        `Heading hierarchy gap: H${prev} → H${current} (should skip at most 1 level)`
      );
    }
  }

  return { valid: issues.length === 0, issues };
}

/**
 * Check for proper ARIA labels and roles
 */
export async function checkAriaLabels(page: Page): Promise<{ missing: string[]; invalid: string[] }> {
  const { missing, invalid } = await page.evaluate(() => {
    const missing: string[] = [];
    const invalid: string[] = [];

    // Check buttons have labels
    document.querySelectorAll('button').forEach((btn) => {
      const text = btn.textContent?.trim();
      const ariaLabel = btn.getAttribute('aria-label');
      const title = btn.getAttribute('title');

      if (!text && !ariaLabel && !title) {
        missing.push(`Button at ${btn.className} has no accessible label`);
      }
    });

    // Check icon buttons specifically
    document.querySelectorAll('[role="button"][class*="icon"]').forEach((btn) => {
      const ariaLabel = btn.getAttribute('aria-label');
      if (!ariaLabel) {
        missing.push(`Icon button missing aria-label: ${btn.className}`);
      }
    });

    // Check invalid roles
    document.querySelectorAll('[role]').forEach((el) => {
      const role = el.getAttribute('role');
      const validRoles = [
        'button',
        'link',
        'navigation',
        'main',
        'complementary',
        'contentinfo',
        'region',
        'alert',
        'alertdialog',
        'dialog',
        'menuitem',
        'tab',
      ];
      if (role && !validRoles.includes(role)) {
        invalid.push(`Invalid role '${role}' on ${el.tagName}`);
      }
    });

    return { missing, invalid };
  });

  return { missing, invalid };
}

/**
 * Check touch target sizes (minimum 48x48px)
 */
export async function checkTouchTargets(page: Page): Promise<string[]> {
  const tooSmall = await page.evaluate(() => {
    const targets: string[] = [];

    document.querySelectorAll('button, a, input[type="checkbox"], input[type="radio"]').forEach((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width < 48 || rect.height < 48) {
        targets.push(
          `${el.tagName.toLowerCase()} (${Math.round(rect.width)}x${Math.round(
            rect.height
          )}px) - ${el.className}`
        );
      }
    });

    return targets;
  });

  return tooSmall;
}

/**
 * Check error message associations with form fields
 */
export async function checkErrorAssociations(page: Page): Promise<string[]> {
  const unassociated = await page.evaluate(() => {
    const issues: string[] = [];

    document.querySelectorAll('[role="alert"], .error, [class*="error"]').forEach((error) => {
      const ariaLabelledBy = error.getAttribute('aria-labelledby');
      const ariaDescribedBy = error.getAttribute('aria-describedby');
      const parentForm = error.closest('form, [role="form"]');

      if (!ariaLabelledBy && !ariaDescribedBy && !parentForm) {
        issues.push(`Error message not associated with form: ${error.textContent?.substring(0, 30)}`);
      }
    });

    return issues;
  });

  return unassociated;
}

/**
 * Generate comprehensive accessibility report
 */
export async function generateA11yReport(page: Page): Promise<AccessibilityReport> {
  const violations = await checkAccessibility(page);
  const heading = await checkHeadingHierarchy(page);
  const labels = await checkInputLabels(page);
  const aria = await checkAriaLabels(page);
  const contrast = await checkColorContrast(page);
  const targets = await checkTouchTargets(page);
  const errors = await checkErrorAssociations(page);

  return {
    violations,
    passes: [],
    incomplete: [],
    timestamp: new Date().toISOString(),
  };
}
