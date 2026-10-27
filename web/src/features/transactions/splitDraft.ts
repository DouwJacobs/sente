import { cents, decimal } from "../../api";
import type { Row } from "../../shared/types";

// Never round through floating point: compute the remainder in integer cents.
// Incomplete/invalid earlier input leaves the last value empty until it is valid.
export function balanceLastSplit(allocations: Row[], amount: string): Row[] {
  if (allocations.length < 2) return allocations;
  let remaining: string;
  try {
    const total = cents(amount);
    const assigned = allocations.slice(0, -1).reduce((sum, allocation) => sum + cents(allocation.amount), 0);
    if (!Number.isSafeInteger(assigned) || !Number.isSafeInteger(total - assigned)) throw new Error("Split total is too large");
    remaining = decimal(total - assigned);
  } catch {
    remaining = "";
  }
  return allocations.map((allocation, index) => index === allocations.length - 1 ? { ...allocation, amount: remaining } : allocation);
}
