// TODO: add a unit test for this hook if/when @testing-library/react is added.
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

type FilterState = {
  selectedDomain: string | null;
};

type FilterCtx = FilterState & {
  setSelectedDomain: (domain: string | null) => void;
  toggleDomain: (domain: string) => void;
  clearFilter: () => void;
};

const Ctx = createContext<FilterCtx | null>(null);

/**
 * Provides cross-panel filter state to every client-detail panel. The
 * provider is mounted once on the ClientDetail page; nested panels use
 * useClientDetailFilter() to read selectedDomain or push a new one.
 */
export function ClientDetailFilterProvider({ children }: { children: ReactNode }) {
  const [selectedDomain, setSelectedDomain] = useState<string | null>(null);

  const toggleDomain = useCallback((domain: string) => {
    setSelectedDomain((current) => (current === domain ? null : domain));
  }, []);

  const clearFilter = useCallback(() => setSelectedDomain(null), []);

  const value = useMemo<FilterCtx>(
    () => ({ selectedDomain, setSelectedDomain, toggleDomain, clearFilter }),
    [selectedDomain, toggleDomain, clearFilter],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

/**
 * Hook for reading + writing the client-detail filter state. Throws when
 * called outside a ClientDetailFilterProvider — this is intentional, the
 * panels are always rendered inside the provider on the ClientDetail page.
 */
export function useClientDetailFilter(): FilterCtx {
  const v = useContext(Ctx);
  if (!v) {
    throw new Error(
      "useClientDetailFilter must be used inside ClientDetailFilterProvider",
    );
  }
  return v;
}
