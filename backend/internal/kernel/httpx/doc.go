// Package httpx is the HTTP transport layer of the Platform.
//
// Layout (one concern per file):
//   - router.go      builds the chi router and mounts routes; no business logic
//   - handlers_*.go  one file per route group; handlers parse input, call a
//     service port, and write a response — nothing else
//   - respond.go     JSON and error-envelope writers shared by all handlers
//   - middleware.go  cross-cutting middleware (request logging, …)
//   - ready.go       readiness probes for the dependencies the process needs
//
// Domain packages never import httpx; httpx depends on them through the small
// interfaces declared in Deps.
package httpx
