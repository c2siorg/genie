// money.ts — the single source of truth for rendering money in Genie.
//
// WHY THIS EXISTS (see plan.md §3.2): the backend stores all amounts in
// *paise* (integer hundredths of a rupee), and Go uses int64. The #1
// catastrophic fintech-UI bug is paise/rupee confusion. So:
//   1. NOTHING in the UI formats money inline — everything goes through here.
//   2. We use BigInt, because int64 paise (up to 9.2e18) exceeds JavaScript's
//      safe-integer range (2^53). Formatting via float would silently lose
//      precision on large settlements.
//   3. Rupees are grouped in the Indian numbering system (1,00,000 not 100,000).

export type PaiseInput = bigint | number | string;

export interface FormatOptions {
  /** Currency symbol/prefix. Default "₹". */
  currency?: string;
  /** If true, prefix non-negative values with "+". Negative always shows "-". */
  showSign?: boolean;
}

/**
 * Coerce a paise value to bigint, rejecting anything that would lose precision
 * or isn't an integer count of paise.
 */
export function toPaise(value: PaiseInput): bigint {
  if (typeof value === "bigint") return value;
  if (typeof value === "number") {
    if (!Number.isInteger(value)) {
      throw new RangeError(`money: paise must be an integer, got ${value}`);
    }
    if (!Number.isSafeInteger(value)) {
      throw new RangeError(
        `money: ${value} exceeds JS safe-integer range; pass paise as a string or bigint`,
      );
    }
    return BigInt(value);
  }
  const trimmed = value.trim();
  if (!/^-?\d+$/.test(trimmed)) {
    throw new TypeError(`money: invalid paise string ${JSON.stringify(value)}`);
  }
  return BigInt(trimmed);
}

/** Group an unsigned integer digit-string using the Indian system (last 3, then 2s). */
export function groupIndian(digits: string): string {
  if (digits.length <= 3) return digits;
  const last3 = digits.slice(-3);
  let rest = digits.slice(0, -3);
  const groups: string[] = [];
  while (rest.length > 2) {
    groups.unshift(rest.slice(-2));
    rest = rest.slice(0, -2);
  }
  if (rest.length > 0) groups.unshift(rest);
  return groups.join(",") + "," + last3;
}

/**
 * Format a paise amount as a rupee string, e.g. 10000000 → "₹1,00,000.00".
 * Always renders exactly two decimal places. Precise for the full int64 range.
 */
export function formatPaise(value: PaiseInput, opts: FormatOptions = {}): string {
  const { currency = "₹", showSign = false } = opts;
  const paise = toPaise(value);
  const negative = paise < 0n;
  const abs = negative ? -paise : paise;
  const rupees = abs / 100n;
  const fraction = abs % 100n;
  const body = `${currency}${groupIndian(rupees.toString())}.${fraction
    .toString()
    .padStart(2, "0")}`;
  if (negative) return `-${body}`;
  if (showSign) return `+${body}`;
  return body;
}
