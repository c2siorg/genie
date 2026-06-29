import { describe, it, expect } from "vitest";
import { formatPaise, groupIndian, toPaise } from "./money";

describe("toPaise", () => {
  it("accepts bigint, number, and string integers", () => {
    expect(toPaise(100n)).toBe(100n);
    expect(toPaise(100)).toBe(100n);
    expect(toPaise("100")).toBe(100n);
    expect(toPaise("-100")).toBe(-100n);
    expect(toPaise(0)).toBe(0n);
  });

  it("rejects non-integer numbers (fractional paise are meaningless)", () => {
    expect(() => toPaise(1.5)).toThrow(RangeError);
  });

  it("rejects numbers beyond the JS safe-integer range (precision risk)", () => {
    expect(() => toPaise(Number.MAX_SAFE_INTEGER + 2)).toThrow(RangeError);
  });

  it("accepts large amounts as strings/bigint without precision loss", () => {
    // int64 max paise.
    expect(toPaise("9223372036854775807")).toBe(9223372036854775807n);
  });

  it("rejects malformed strings", () => {
    expect(() => toPaise("1.00")).toThrow(TypeError);
    expect(() => toPaise("₹100")).toThrow(TypeError);
    expect(() => toPaise("abc")).toThrow(TypeError);
    expect(() => toPaise("")).toThrow(TypeError);
  });
});

describe("groupIndian", () => {
  it("leaves <=3 digit numbers ungrouped", () => {
    expect(groupIndian("0")).toBe("0");
    expect(groupIndian("999")).toBe("999");
  });
  it("groups the last 3 then by 2s (Indian system)", () => {
    expect(groupIndian("1000")).toBe("1,000");
    expect(groupIndian("100000")).toBe("1,00,000"); // 1 lakh
    expect(groupIndian("10000000")).toBe("1,00,00,000"); // 1 crore
    expect(groupIndian("1000000000")).toBe("1,00,00,00,000");
  });
});

describe("formatPaise", () => {
  it("formats boundary small values with two decimals", () => {
    expect(formatPaise(0)).toBe("₹0.00");
    expect(formatPaise(1)).toBe("₹0.01");
    expect(formatPaise(9)).toBe("₹0.09");
    expect(formatPaise(99)).toBe("₹0.99");
    expect(formatPaise(100)).toBe("₹1.00");
    expect(formatPaise(12345)).toBe("₹123.45");
  });

  it("applies Indian grouping to the rupee part", () => {
    expect(formatPaise(100000)).toBe("₹1,000.00"); // ₹1,000
    expect(formatPaise(10000000)).toBe("₹1,00,000.00"); // ₹1 lakh
    expect(formatPaise(1000000000)).toBe("₹1,00,00,000.00"); // ₹1 crore
  });

  it("handles negative amounts (e.g. refunds)", () => {
    expect(formatPaise(-100)).toBe("-₹1.00");
    expect(formatPaise(-1)).toBe("-₹0.01");
  });

  it("honours showSign for non-negative values", () => {
    expect(formatPaise(100, { showSign: true })).toBe("+₹1.00");
    expect(formatPaise(0, { showSign: true })).toBe("+₹0.00");
    expect(formatPaise(-100, { showSign: true })).toBe("-₹1.00");
  });

  it("supports an alternate currency prefix", () => {
    expect(formatPaise(100, { currency: "INR " })).toBe("INR 1.00");
  });

  it("is precise at the int64 maximum (no float rounding)", () => {
    // 9223372036854775807 paise = ₹92,23,37,20,36,85,47,758.07
    expect(formatPaise("9223372036854775807")).toBe(
      "₹92,23,37,20,36,85,47,758.07",
    );
  });

  it("accepts bigint input", () => {
    expect(formatPaise(2500n)).toBe("₹25.00");
  });
});
