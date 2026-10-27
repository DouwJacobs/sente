import { describe, expect, it } from "vitest";
import { balanceLastSplit } from "./splitDraft";
const rows = (...amounts: string[]) => amounts.map(amount => ({ amount }));
describe("split remainder", () => {
 it("balances two and three-way signed amounts exactly", () => {
  expect(balanceLastSplit(rows("-300", "0"), "-500")[1].amount).toBe("-200.00");
  expect(balanceLastSplit(rows("-300", "-125.45", "0"), "-500")[2].amount).toBe("-74.55");
  expect(balanceLastSplit(rows("0.10", "0.20", "0"), "0.60")[2].amount).toBe("0.30");
 });
 it("clears stale remainder during incomplete input and preserves invalid over-allocation for validation", () => {
  expect(balanceLastSplit(rows("-", "-200"), "-500")[1].amount).toBe("");
  expect(balanceLastSplit(rows("-600", "0"), "-500")[1].amount).toBe("100.00");
 });
 it("rebalances after removal and total changes without changing classification", () => {
  const draft = [{ amount: "-300", category_id: 1, spending_group_override: 2, note: "Keep" }, { amount: "0", category_id: 2 }];
  expect(balanceLastSplit(draft, "-600")).toEqual([draft[0], { ...draft[1], amount: "-300.00" }]);
  expect(balanceLastSplit(rows("-500"), "-500")[0].amount).toBe("-500");
 });
});
