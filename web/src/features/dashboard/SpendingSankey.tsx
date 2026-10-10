import { useEffect, useId, useRef, useState } from "react";
import { money } from "../../api";
import { Empty } from "../../ui";
import { spendingFlow, spendingPercent, type SpendingFlowEntry } from "./spendingFlow";

function band(x1: number, y1: number, x2: number, y2: number, height: number) {
  const mid = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2} ${y2} L ${x2} ${y2 + height} C ${mid} ${y2 + height}, ${mid} ${y1 + height}, ${x1} ${y1 + height} Z`;
}

function NodeLabel({ x, y, width, name, cents }: { x: number; y: number; width: number; name: string; cents: number }) {
  return (
    <foreignObject x={x} y={y - 21} width={width} height="42">
      <div className="sankey-label" title={name + ": " + money(cents)}>
        <strong>{name}</strong><span>{money(cents)}</span>
      </div>
    </foreignObject>
  );
}

export function SpendingSankey({ entries, total }: { entries: SpendingFlowEntry[]; total: number }) {
  const id = useId();
  const container = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(900);
  useEffect(() => {
    if (!container.current) return;
    const observer = new ResizeObserver(([entry]) => setWidth(Math.max(240, entry.contentRect.width)));
    observer.observe(container.current);
    return () => observer.disconnect();
  }, []);
  const flow = spendingFlow(entries);
  const compact = width < 620;
  // Keep the complete hierarchy: natural height replaces scrolling, and labels
  // belong to their actual node rather than floating over an earlier ribbon.
  const count = flow.groups.reduce((sum, group) => sum + group.entries.length, 0);
  const gap = 48;
  const height = Math.max(320, count * 60 + flow.groups.length * 16 + 40);
  const space = height - 40 - Math.max(0, count - 1) * gap - Math.max(0, flow.groups.length - 1) * 16;
  const scale = flow.positive > 0 ? space / flow.positive : 0;
  const rootX = 8;
  const groupX = compact ? 8 : width * .35;
  const categoryX = width * (compact ? .56 : .72);
  const groupLabelWidth = Math.max(82, Math.min(160, categoryX - groupX - 32));
  const categoryLabelWidth = width - categoryX - 24;
  const detail = (name: string, cents: number) =>
    `${name}: ${money(cents)} · ${spendingPercent(cents, total)} of net period spending`;
  let categoryY = 20, rootOffset = 0;
  const geometry = flow.groups.map(group => {
    const start = categoryY;
    const children = group.entries.map(entry => {
      const child = { entry, y: categoryY, height: entry.spent_cents * scale };
      categoryY += child.height + gap;
      return child;
    });
    const groupHeight = group.cents * scale;
    const y = start + (categoryY - gap - start - groupHeight) / 2;
    const rootY = 20 + rootOffset;
    rootOffset += groupHeight;
    categoryY += 16;
    return { group, y, height: groupHeight, rootY, children };
  });

  return (
    <section className="panel spending-sankey" aria-labelledby={id}>
      <div className="section-head">
        <div>
          <h2 id={id}>Spending flow</h2>
          <p className="muted">Total spending → Spending groups → Categories</p>
        </div>
        <strong className="sankey-total">Total spending: {money(total)}</strong>
      </div>
      {flow.refunds > 0 && (
        <p className="footnote">
          Positive category flows total {money(flow.positive)}. Net refunds of {money(flow.refunds)} reduce period spending to {money(total)}.
          Negative and zero category totals appear in the breakdown below; they have no ribbon.
          Percentages use net period spending and can exceed 100%.
        </p>
      )}
      <div ref={container} className="sankey-chart">
        {flow.positive > 0 ? (
          <svg viewBox={`0 0 ${width} ${height}`} width={width} height={height} role="img" aria-label={`Spending flows for this period. Net spending ${money(total)}. See Spending breakdown for full amounts and percentages.`}>
            {geometry.map(({ group, y, height: size, rootY, children }) => {
              let offset = 0;
              return (
                <g key={group.id}>
                  {!compact && <path className="sankey-band" d={band(rootX + 12, rootY, groupX, y, size)}>
                    <title>{detail(group.name + " positive flows", group.cents)}</title>
                  </path>}
                  {children.map(child => {
                    const source = y + offset;
                    offset += child.height;
                    return (
                      <g key={child.entry.category_id}>
                        <path className="sankey-band" d={band(groupX + 12, source, categoryX, child.y, child.height)}>
                          <title>{detail(group.name + " → " + child.entry.category_name, child.entry.spent_cents)}</title>
                        </path>
                        <rect className="sankey-node" x={categoryX} y={child.y} width="12" height={child.height} data-cents={child.entry.spent_cents}>
                          <title>{detail(group.name + " → " + child.entry.category_name, child.entry.spent_cents)}</title>
                        </rect>
                      </g>
                    );
                  })}
                  <rect className="sankey-node" x={groupX} y={y} width="12" height={size} data-group={group.name} data-cents={group.cents}>
                    <title>{detail(group.name + " positive flows", group.cents)}</title>
                  </rect>
                </g>
              );
            })}
            {!compact && <rect className="sankey-node" x={rootX} y="20" width="12" height={flow.positive * scale}>
              <title>{detail(flow.refunds ? "Positive category flows" : "Total spending", flow.positive)}</title>
            </rect>}
            {/* Labels render last, immediately beside their node's centre. */}
            {!compact && <NodeLabel x={rootX + 20} y={20 + flow.positive * scale / 2} width={Math.max(90, groupX - 40)} name={flow.refunds ? "Positive flows" : "Total spending"} cents={flow.positive} />}
            {geometry.map(({ group, y, height: size, children }) => (
              <g key={group.id}>
                <NodeLabel x={groupX + 20} y={y + size / 2} width={groupLabelWidth} name={group.name} cents={group.cents} />
                {children.map(child => <NodeLabel key={child.entry.category_id} x={categoryX + 20} y={child.y + child.height / 2} width={categoryLabelWidth} name={child.entry.category_name} cents={child.entry.spent_cents} />)}
              </g>
            ))}
          </svg>
        ) : (
          <Empty kind="budget" title={entries.length ? "No positive spending flows" : "No spending yet"}>
            <p>{entries.length ? "Refunds and zero totals have no spending ribbon. See the net amounts below." : "Spending flows will appear when expenses are recorded for this period."}</p>
          </Empty>
        )}
      </div>
      {entries.length > 0 && (
        <details className="sankey-breakdown">
          <summary>Spending breakdown</summary>
          <p className="footnote">Amounts include refunds and uncategorised expenses. Transfers are excluded. Percentages are unavailable when net spending is zero or negative.</p>
          <table>
            <caption className="sr-only">Spending by group and category for the selected period and accounts</caption>
            <thead><tr><th scope="col">Spending group</th><th scope="col">Category</th><th scope="col">Net spending</th><th scope="col">Of period total</th></tr></thead>
            <tbody>{entries.map(entry => (
              <tr key={entry.group_id + ":" + entry.category_id}>
                <th scope="row">{entry.group_name}</th>
                <td data-label="Category">{entry.category_name}</td>
                <td data-label="Net spending">{money(entry.spent_cents)}</td>
                <td data-label="Of period total">{spendingPercent(entry.spent_cents, total)}</td>
              </tr>
            ))}</tbody>
            <tfoot><tr><th scope="row" colSpan={2}>Total spending</th><td data-label="Net spending">{money(total)}</td><td data-label="Of period total">{spendingPercent(total, total)}</td></tr></tfoot>
          </table>
        </details>
      )}
    </section>
  );
}
