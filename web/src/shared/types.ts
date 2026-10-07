export type Row = Record<string, any>;
export type Data = {
  user: Row;
  accounts: Row[];
  categories: Row[];
  periods: Row[];
  rules: Row[];
  spendingGroups: Row[];
  branding: Row;
  next: Row;
};
export type PageProps = {
  data: Data;
  revision: number;
  refresh: () => void;
  notify: (message: string, error?: boolean) => void;
};
