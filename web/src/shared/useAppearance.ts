import { useEffect, useState } from "react";
export function useAppearance() {
  const [theme, setTheme] = useState(
    () => localStorage.getItem("finance-theme") || "system",
  );
  const [resolvedTheme, setResolvedTheme] = useState(() =>
    matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
  );
  useEffect(() => {
    const query = matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const resolved =
        theme === "system" ? (query.matches ? "dark" : "light") : theme;
      document.documentElement.dataset.theme = resolved;
      setResolvedTheme(resolved);
    };
    apply();
    query.addEventListener("change", apply);
    localStorage.setItem("finance-theme", theme);
    return () => query.removeEventListener("change", apply);
  }, [theme]);
  return { theme, setTheme, resolvedTheme };
}
