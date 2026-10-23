// Package holds decides when a driver may not go online and records why.
//
// Two independent kinds of hold:
//   - DEBT holds follow the debt rule (rules.go): owing at least the hard limit,
//     or at least the soft limit for longer than the grace days. They are placed
//     and lifted only by Evaluate, never by staff.
//   - MANUAL holds are placed and released by staff with a reason (audited).
//
// Files:
//   - model.go   hold kinds, causes, the Hold type and error codes
//   - rules.go   the pure debt rule (Decide)
//   - ports.go   what this module needs from money, pricing and presence
//   - store.go   driver_holds and driver_debt_clock persistence
//   - service.go use cases: Evaluate, Enforce, Place, Release, Active, ListActive
//   - jobs.go    the periodic sweep that makes overdue soft holds fire on time
package holds
