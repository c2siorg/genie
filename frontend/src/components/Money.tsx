import { formatPaise, type FormatOptions, type PaiseInput } from "../lib/money";

export interface MoneyProps extends FormatOptions {
  /** Amount in paise (integer). Use string/bigint for values above 2^53. */
  paise: PaiseInput;
  /**
   * Semantic intent. "credit" renders positive/green, "debit" red. "neutral"
   * (default) is plain. This drives a data attribute + class, not inline color,
   * so theming lives in tokens.css.
   */
  intent?: "neutral" | "credit" | "debit";
  className?: string;
}

/**
 * Money is the ONLY sanctioned way to render an amount in the Genie UI.
 * It renders a paise value as grouped rupees via the audited formatPaise()
 * helper and exposes the raw paise on a data attribute for tests/automation.
 *
 * Rendering rules (see plan.md §3.2): always unambiguous, always currency-
 * prefixed, never hand-formatted at a call site.
 */
export function Money({
  paise,
  intent = "neutral",
  currency,
  showSign,
  className,
}: MoneyProps) {
  const text = formatPaise(paise, { currency, showSign });
  const cls = ["money", `money--${intent}`, className].filter(Boolean).join(" ");
  return (
    <span
      className={cls}
      data-paise={String(paise)}
      data-intent={intent}
      // The grouped string is decorative for SR users who'd hear digits oddly;
      // expose a clean label.
      aria-label={text}
    >
      {text}
    </span>
  );
}
