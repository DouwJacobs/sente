import { describe, expect, it } from "vitest";
import { capabilityLabels, reviewPermissions, selectPreset } from "./MCPPermissions";
describe("explicit MCP permission presets", () => {
 it("enables all supported capabilities and reads only through YOLO selection", () => {
  const initial = reviewPermissions();
  const yolo = selectPreset({...initial, constraints: {...initial.constraints, selected_accounts: true, account_ids: [7]}}, "yolo");
  expect(Object.keys(yolo.capabilities)).toEqual(Object.keys(capabilityLabels));
  expect(Object.values(yolo.capabilities).every(Boolean)).toBe(true);
  expect(yolo.automatic_approval).toEqual(yolo.capabilities);
  expect([yolo.read_context,yolo.read_merchants,yolo.read_budget_alerts]).toEqual([true,true,true]);
  expect(yolo.constraints.account_ids).toEqual([]);
  expect(yolo.constraints.selected_accounts).toBe(false);
  expect(selectPreset(yolo,"review").automatic_approval).toEqual({});
  expect(selectPreset(initial,"finance").capabilities.create_period).toBe(false);
  expect(initial.read_context).toBe(false);
 });
});
