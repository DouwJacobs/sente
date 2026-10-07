import {
  useEffect,
  useState,
  useRef,
  type Dispatch,
  type SetStateAction,
} from "react";
import { api, setCSRF } from "../../api";
import type { Data, Row, PageProps } from "../../shared/types";
export function useWorkspaceData({
  user,
  setUser,
  revision,
  consent,
  account,
  setAccount,
  period,
  setPeriod,
  notify,
}: {
  user: Row | null;
  setUser: Dispatch<SetStateAction<Row | null>>;
  revision: number;
  consent: boolean;
  account: string;
  setAccount: Dispatch<SetStateAction<string>>;
  period: string;
  setPeriod: Dispatch<SetStateAction<string>>;
  notify: PageProps["notify"];
}) {
  const [data, setData] = useState<Data | null>(null);
  const [refreshingData, setRefreshingData] = useState(false),
    periodInitialized = useRef(false);
  const [workCounts, setWorkCounts] = useState({ review: 0, imports: 0 });
  useEffect(() => {
    if (!user || consent) return;
    let alive = true;
    setRefreshingData(true);
    api("/me")
      .then((session) => {
        if (!alive) return null;
        setCSRF(session.csrf);
        setUser((previous) =>
          previous &&
          ["id", "username", "admin", "budget_member"].every(
            (key) => previous[key] === session.user[key],
          )
            ? previous
            : session.user,
        );
        return Promise.all([
          api("/accounts?page=0&page_size=100"),
          api("/categories?page=0&page_size=100"),
          session.user.budget_member
            ? api("/periods?page=0&page_size=100")
            : Promise.resolve({ items: [], next: {} }),
          Promise.resolve([]),
          api("/spending-groups?page=0&page_size=100"),
          api("/branding"),
          Promise.resolve(session.user),
        ]);
      })
      .then(async (result) => {
        if (!alive || !result) return;
        const [
          accounts,
          categories,
          periods,
          rules,
          spendingGroups,
          branding,
          latestUser,
        ] = result;
        if (
          account &&
          !accounts.items.some((a: Row) => String(a.id) === account)
        ) {
          const v = await api("/accounts?page=0&id=" + account);
          accounts.items.push(...v.items);
        }
        if (
          period &&
          latestUser.budget_member &&
          !periods.items.some((p: Row) => String(p.id) === period)
        ) {
          const v = await api("/periods?page=0&id=" + period);
          periods.items.push(...v.items);
        }
        if (!alive) return;
        setData({
          user: latestUser,
          accounts: accounts.items,
          categories: categories.items,
          periods: periods.items,
          next: periods.next,
          rules,
          spendingGroups: spendingGroups.items,
          branding,
        });
        const today = new Intl.DateTimeFormat("en-CA", {
          timeZone: "Africa/Johannesburg",
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
        }).format(new Date());
        const current = periods.items.find(
          (p: Row) => p.start_date <= today && p.end_date >= today,
        );
        if (!periodInitialized.current) {
          periodInitialized.current = true;
          setPeriod(String(current?.id || periods.items[0]?.id || ""));
        } else
          setPeriod((old) =>
            !old || periods.items.some((p: Row) => String(p.id) === old)
              ? old
              : "",
          );
        setAccount((old) =>
          accounts.items.some((a: Row) => String(a.id) === old) ? old : "",
        );
      })
      .catch((e) => alive && notify(e.message, true))
      .finally(() => alive && setRefreshingData(false));
    return () => {
      alive = false;
    };
  }, [user, revision]);
  useEffect(() => {
    if (!user || consent) return;
    let alive = true;
    Promise.all([api("/transactions?pending=1"), api("/imports?page=0")])
      .then(([t, i]) => {
        if (alive)
          setWorkCounts({ review: t.total, imports: i.pending_total || 0 });
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [user, revision]);
  return { data, setData, refreshingData, workCounts };
}
