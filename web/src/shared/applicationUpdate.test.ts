import { describe, expect, it } from "vitest";
import { updateLabel, type ApplicationUpdate } from "./applicationUpdate";
describe("public application update labels", () => {
  it("never treats unavailable, unknown or unsupported checks as current", () => {
    for (const state of ["unavailable", "unknown", "unsupported"] as const) {
      expect(updateLabel({ state, stale: true })).not.toContain("Up to date");
    }
    expect(updateLabel(null)).toContain("Checking");
  });
  it("reports the candidate only when the server establishes precedence", () => {
    const status: ApplicationUpdate = { state: "available", stale: false, available_version: "1.0.0-beta.10" };
    expect(updateLabel(status)).toBe("Update available: 1.0.0-beta.10");
    expect(updateLabel({ ...status, state: "unavailable", stale: true })).toBe("Release check unavailable");
  });
});
