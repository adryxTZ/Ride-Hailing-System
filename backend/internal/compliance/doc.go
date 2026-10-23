// Package compliance keeps the Platform on the right side of each country's
// rules: driver documents stay valid (expiry reminders and holds), and the
// transport regulator hears what it must: drivers and vehicles are registered when staff approve
// them, every eligible completed trip is submitted once, and a daily report is
// sent per country day.
//
// Everything goes through one outbox (regulator_submissions) and one clock: the
// sweeper claims due rows, sends them through the regulator port, and retries
// on a fixed ladder (1m, 5m, 30m, 3h, then daily) until the regulator accepts.
// Nothing is ever abandoned; a row is flagged stuck after StuckAfterAttempts.
// Rows are queued even while sending is off, so the backlog drains later.
//
// Files:
//   - model.go    Submission, statuses, kinds, timing constants, error codes
//   - ladder.go   pure rules: NextAttempt, ShouldSubmit, outcome of an answer
//   - store.go    regulator_submissions and regulator_settings persistence
//   - records.go  building regulator records from the database at send time
//   - service.go  enqueueing (in the caller's transaction), settings, retry, lists, the fail-closed gate
//   - sweep.go    the sweeper and the daily report
//   - documents.go       document expiry: reminders at 30/7/1 days (once each) and holds for expired blocking documents
//   - documents_store.go the documents query behind it
//   - jobs.go     River workers
package compliance
