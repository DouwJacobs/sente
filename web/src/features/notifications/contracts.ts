export type Notification = {
  id: number; type: string; severity: string; title: string; message: string;
  source_kind: "system" | "budget" | "account" | "transaction";
  source_id: number; occurred_at: number; created_at: number;
  read_at: number | null; dismissible: number;
};
export type Inbox = { items: Notification[]; total: number; unread_count: number };
export type Preference = { type: string; channel: string; enabled: boolean; version: number };
export const notificationLabels: Record<string, string> = {
  system: "System updates", budget_threshold: "Budget thresholds",
  budget_projection: "Projected overspend", budget_overspend: "Budget overspend",
  unusual_spending: "Unusual spending", recurring_payment: "Recurring payment changes",
};
