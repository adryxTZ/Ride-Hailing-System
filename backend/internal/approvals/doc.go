// Package approvals is configurable approval workflows for sensitive staff
// actions (spec DOC/specs/2026-10-24-approval-workflows.md).
//
// The wiring registers approvable actions in a Catalogue: a key, labels, the
// permission that may ask for it, an optional numeric condition field
// (amount), a Describer that turns the stored payload into a readable diff and
// an Executor that performs the real change from that payload. A gated HTTP
// endpoint calls Submit: when the action's workflow is enabled and its
// conditions match, the change is stored as a request (202 to the client)
// instead of being applied. Approvers decide step by step; the requester can
// never approve their own request and a person decides a request once. On the
// last approval the executor runs once, as the requester of record, with
// "(approved by …)" in the reason. Workflows are data, versioned and audited;
// a request keeps the definition it started under. Every workflow is off
// until an admin saves one enabled.
//
//   - model.go       actions, workflows, requests, errors
//   - catalogue.go   the registry of approvable actions
//   - conditions.go  when a workflow applies, and checking an edited definition
//   - steps.go       who may approve a step and when it is met
//   - service.go     Submit (the gate) and the workflow cache
//   - workflows.go   list, read and save workflows (versioned)
//   - requests.go    the inbox: list, count, read
//   - decide.go      approve, reject, cancel
//   - execute.go     run an approved action once and record the outcome
//   - expire.go      close overdue requests (River periodic job)
//   - diff.go        field-by-field diffs for summaries
//   - ports.go       what the module needs from identity and the brand
//   - store.go       persistence
//   - actions/       the Platform's approvable actions (adapters to the modules)
package approvals
