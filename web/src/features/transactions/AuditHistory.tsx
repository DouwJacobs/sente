import { usePagedList, ListStatus, ListNavigation } from "../../PagedList";
export function AuditHistory({ id }: { id: number }) {
  const list = usePagedList("/audit/" + id);
  return (
    <>
      <ListStatus list={list} />
      {list.items.map((h) => (
        <div className="history-row" key={h.id}>
          <strong>{h.action.replaceAll("_", " ")}</strong>
          <small>
            {h.created_at} UTC · {h.username || "System"}
          </small>
        </div>
      ))}
      <ListNavigation list={list} />
    </>
  );
}
