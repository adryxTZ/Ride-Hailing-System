// Package fleet owns who may drive: driver profiles, their vehicles, the
// documents that prove both, and the staff review that approves them. A driver
// goes online only with an APPROVED profile and an APPROVED vehicle; the
// services and classes they can serve come from that vehicle, never from the app.
//
//   - model.go       Profile, Vehicle, Document, Onboarding, inputs, statuses, errors
//   - requirements.go which documents are required and what is still missing
//   - ports.go       what fleet needs: the catalogue (class checks), file storage, presence
//   - store.go       queries (profiles, vehicles, documents, audit log)
//   - onboarding.go  the driver's own use cases: profile, vehicle, documents, uploads, submit
//   - review.go      staff use cases: list, detail, approve / reject / suspend, document review
//   - eligibility.go what an approved driver may go online with
package fleet
