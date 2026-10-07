import { describe, expect, it } from "vitest";
import { buildLabel, issueReportURL } from "./buildInfo";

describe("public build information", () => {
  it("keeps extra runtime and financial fields out of issue reports", () => {
    const input = { version: "v1.2.3", commit: "abc123", modified: false,
      built_at: "2026-10-07T12:00:00Z", username: "private-user",
      account: "private-account", notes: "private-note", amount_cents: 987654321 };
    const url = new URL(issueReportURL(input));
    expect(url.origin + url.pathname).toBe("https://github.com/DouwJacobs/sente/issues/new");
    const body = url.searchParams.get("body")!;
    expect(body).toContain("Version: v1.2.3");
    expect(body).toContain("Commit: abc123");
    for (const privateValue of [input.username, input.account, input.notes, String(input.amount_cents)]) {
      expect(body).not.toContain(privateValue);
    }
    expect([...url.searchParams.keys()]).toEqual(["body"]);
  });
  it("keeps issue reporting available without metadata", () => {
    expect(new URL(issueReportURL(null)).searchParams.get("body")).toContain("Build information unavailable");
    expect(buildLabel({ version: "dev", commit: "1234567890123456", modified: true })).toBe("dev (123456789012) · modified");
  });
});
