export type SpendingFlowEntry = {
  group_id: number;
  group_name: string;
  category_id: number;
  category_name: string;
  spent_cents: number;
};

export function spendingFlow(entries: SpendingFlowEntry[]) {
  const groups = new Map<number, { id: number; name: string; cents: number; entries: SpendingFlowEntry[] }>();
  let net = 0, positive = 0, refunds = 0;
  for (const entry of entries) {
    net += entry.spent_cents;
    if (entry.spent_cents < 0) refunds -= entry.spent_cents;
    if (entry.spent_cents <= 0) continue;
    positive += entry.spent_cents;
    let group = groups.get(entry.group_id);
    if (!group) {
      group = { id: entry.group_id, name: entry.group_name, cents: 0, entries: [] };
      groups.set(entry.group_id, group);
    }
    group.cents += entry.spent_cents;
    group.entries.push(entry);
  }
  return { net, positive, refunds, groups: [...groups.values()] };
}

export function spendingPercent(cents: number, total: number) {
  return total > 0 ? new Intl.NumberFormat("en-ZA", { style: "percent", maximumFractionDigits: 1 }).format(cents / total) : "—";
}
