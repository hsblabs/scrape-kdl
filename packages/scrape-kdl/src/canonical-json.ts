export function canonicalJSONStringify(value: unknown): string {
  if (value === null) return "null";
  switch (typeof value) {
    case "boolean": return value ? "true" : "false";
    case "string": return JSON.stringify(value);
    case "number": return canonicalNumber(value);
    case "object": {
      if (Array.isArray(value)) return `[${value.map(canonicalJSONStringify).join(",")}]`;
      const object = value as Record<string, unknown>;
      const keys = Object.keys(object).sort(compareCodePoints);
      return `{${keys.map((key) => `${JSON.stringify(key)}:${canonicalJSONStringify(object[key])}`).join(",")}}`;
    }
    default:
      throw new TypeError(`canonical JSON does not support ${typeof value}`);
  }
}

function compareCodePoints(left: string, right: string): number {
  const leftPoints = Array.from(left, (character) => character.codePointAt(0)!);
  const rightPoints = Array.from(right, (character) => character.codePointAt(0)!);
  const length = Math.min(leftPoints.length, rightPoints.length);
  for (let index = 0; index < length; index++) {
    if (leftPoints[index] !== rightPoints[index]) return leftPoints[index]! - rightPoints[index]!;
  }
  return leftPoints.length - rightPoints.length;
}

function canonicalNumber(value: number): string {
  if (!Number.isFinite(value)) throw new TypeError("canonical JSON rejects non-finite numbers");
  if (Object.is(value, -0)) return "0";
  const raw = String(value);
  if (!/[eE]/u.test(raw)) return raw;
  const [mantissa = "", exponentText = "0"] = raw.toLowerCase().split("e");
  const sign = mantissa.startsWith("-") ? "-" : "";
  const unsigned = sign === "" ? mantissa : mantissa.slice(1);
  const point = unsigned.indexOf(".");
  const digits = unsigned.replace(".", "").replace(/^0+/u, "") || "0";
  const fractionDigits = point < 0 ? 0 : unsigned.length - point - 1;
  const exponent = Number(exponentText) - fractionDigits;
  if (digits === "0") return "0";
  if (exponent >= 0) return sign + digits + "0".repeat(exponent);
  const decimalAt = digits.length + exponent;
  if (decimalAt > 0) return `${sign}${digits.slice(0, decimalAt)}.${digits.slice(decimalAt)}`;
  return `${sign}0.${"0".repeat(-decimalAt)}${digits}`;
}
