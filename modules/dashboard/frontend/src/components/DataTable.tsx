import { useState, useMemo, useCallback, type ReactNode } from "react";
import { cn } from "@/lib/utils";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/ui/table";

export type Column<R> = {
  key: string;
  label: string;
  render: (row: R) => ReactNode;
  sortValue?: (row: R) => string | number;
  className?: string;
};

type SortDir = "asc" | "desc";

export function DataTable<R>({
  columns,
  rows,
  rowKey,
  className,
}: {
  columns: Column<R>[];
  rows: R[];
  rowKey: (row: R) => string;
  className?: string;
}) {
  const [sortCol, setSortCol] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const toggle = useCallback(
    (key: string) => {
      if (sortCol === key) {
        setSortDir((d) => (d === "asc" ? "desc" : "asc"));
      } else {
        setSortCol(key);
        setSortDir("asc");
      }
    },
    [sortCol],
  );

  const sorted = useMemo(() => {
    const col = columns.find((c) => c.key === sortCol);
    if (!col?.sortValue) return rows;
    const cmp = col.sortValue;
    const dir = sortDir === "asc" ? 1 : -1;
    return [...rows].sort((a, b) => {
      const av = cmp(a);
      const bv = cmp(b);
      return av < bv ? -dir : av > bv ? dir : 0;
    });
  }, [rows, columns, sortCol, sortDir]);

  return (
    <Table className={className}>
      <TableHeader>
        <TableRow className="border-b-0 bg-surface-low hover:bg-surface-low">
          {columns.map((col) => (
            <TableHead
              key={col.key}
              className={cn(
                "label-xs cursor-default select-none h-8 px-3",
                col.sortValue && "cursor-pointer",
                col.className,
              )}
              onClick={col.sortValue ? () => toggle(col.key) : undefined}
            >
              {col.label}
              {sortCol === col.key && (sortDir === "asc" ? " \u2191" : " \u2193")}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {sorted.map((row) => (
          <TableRow key={rowKey(row)} className="border-b-0">
            {columns.map((col) => (
              <TableCell key={col.key} className={cn("px-3 py-2", col.className)}>
                {col.render(row)}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
