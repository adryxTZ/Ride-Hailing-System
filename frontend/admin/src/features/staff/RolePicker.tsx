/** RolePicker — choose any number of staff roles (a person can hold several). */
export function RolePicker({
  roles,
  value,
  onChange,
}: {
  roles: string[];
  value: string[];
  onChange: (next: string[]) => void;
}) {
  const toggle = (role: string) =>
    onChange(value.includes(role) ? value.filter((r) => r !== role) : [...value, role]);
  return (
    <div className="flex flex-wrap gap-2" role="group" aria-label="Roles">
      {roles.map((r) => (
        <button
          key={r}
          type="button"
          aria-pressed={value.includes(r)}
          onClick={() => toggle(r)}
          className={`rounded-[8px] px-3 py-1.5 text-sm font-medium transition-colors ${value.includes(r) ? "bg-nav text-nav-ink" : "bg-surface-2 text-muted hover:text-ink"}`}
        >
          {r}
        </button>
      ))}
    </div>
  );
}
