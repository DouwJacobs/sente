import { describe, expect, it } from "vitest";
import { spendingFlow, spendingPercent, type SpendingFlowEntry } from "./spendingFlow";

const entry = (group: number, category: number, cents: number): SpendingFlowEntry => ({
  group_id: group, group_name: "Group " + group, category_id: category,
  category_name: category ? "Category " + category : "Uncategorised", spent_cents: cents,
});
describe("spending flow financial reconciliation", () => {
  it("keeps the same category separate in different groups and includes uncategorised spending", () => {
    const flow = spendingFlow([entry(1, 1, 900), entry(2, 1, 500), entry(0, 0, 200)]);
    expect(flow.net).toBe(1600);
    expect(flow.positive).toBe(1600);
    expect(flow.groups.map(g => g.cents)).toEqual([900, 500, 200]);
    expect(flow.groups[2].entries[0].category_id).toBe(0);
  });
  it("reconciles refunds without negative or zero ribbons", () => {
    const flow = spendingFlow([entry(1, 1, 1000), entry(1, 2, -400), entry(2, 3, 0)]);
    expect(flow.net).toBe(600);
    expect(flow.positive - flow.refunds).toBe(flow.net);
    expect(flow.groups).toHaveLength(1);
    expect(flow.groups[0].entries).toHaveLength(1);
    expect(spendingPercent(1000, flow.net)).toBe("166,7%");
    expect(spendingPercent(-400, flow.net)).toBe("-66,7%");
  });
  it("handles empty, fully refunded and negative net spending", () => {
    for (const entries of [[], [entry(1, 1, 0)], [entry(1, 1, -100)]]) {
      const flow = spendingFlow(entries);
      expect(flow.groups).toEqual([]);
      expect(flow.positive).toBe(0);
      expect(spendingPercent(100, flow.net)).toBe("—");
    }
    const flow = spendingFlow([entry(1, 1, 100), entry(2, 1, -200)]);
    expect(flow.positive - flow.refunds).toBe(-100);
    expect(spendingPercent(100, flow.net)).toBe("—");
  });
});
