/** Table — dense rows with hairlines between them; header in small muted caps. */
export function Table({ head, children }: { head: string[]; children: React.ReactNode }) {
  return (
    <div className="-mx-2 max-h-[600px] overflow-auto rounded-lg border border-line">
      <table className="w-full min-w-[640px] border-collapse text-sm">
        <thead className="sticky top-0 z-10 bg-surface-2">
          <tr>
            {head.map((h) => (
              <th
                key={h}
                scope="col"
                className="border-b border-line px-3 py-3 text-left text-[10px] font-semibold uppercase tracking-wider text-muted"
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

export function Row({
  children,
  onClick,
  selected,
}: {
  children: React.ReactNode;
  onClick?: () => void;
  selected?: boolean;
}) {
  return (
    <tr
      onClick={onClick}
      onKeyDown={onClick ? (e) => (e.key === "Enter" ? onClick() : undefined) : undefined}
      tabIndex={onClick ? 0 : undefined}
      className={`border-t border-line transition-colors duration-150 ${onClick ? "cursor-pointer hover:bg-surface-2" : ""} ${selected ? "bg-surface-2" : ""}`}
    >
      {children}
    </tr>
  );
}

export function Cell({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <td className={`px-3 py-3.5 align-middle ${className}`}>{children}</td>;
}
