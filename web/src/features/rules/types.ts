import type { Row } from "../../shared/types";
export type RuleRef = {
  kind: "merchant" | "category";
  id: number;
  version: number;
};
export const ruleRefs = (rule: Row): RuleRef[] =>
  rule.refs ||
  (rule.rows || [rule]).map((r: Row) => ({
    kind: r.rule_kind || (r.merchant_id ? "merchant" : "category"),
    id: r.id,
    version: r.version,
  }));
export function rulePayload(rule: Row) {
  const kind = rule.rule_kind || (rule.merchant_id ? "merchant" : "category");
  const common = {
    account_id: Number(rule.account_id || 0),
    pattern: rule.pattern,
    direction: rule.direction,
    priority: Number(rule.priority),
    enabled: !!rule.enabled,
    spending_group_id: rule.spending_group_id || null,
  };
  return {
    kind,
    refs: ruleRefs(rule),
    rule: { ...common, category_id: Number(rule.category_id || 0) },
    merchant_rule: {
      ...common,
      merchant_id: rule.merchant_id || 0,
      category_id: rule.category_id || null,
      clear_category: !rule.category_id,
      clear_group: !rule.spending_group_id,
    },
  };
}
