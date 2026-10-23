// Package messaging owns what people send each other around a trip and what
// the Platform sends to their phones: in-trip chat, the two ratings after a
// trip, push devices and push delivery.
//
//   - model.go     Message, Rating, Device, quick replies, errors
//   - ports.go     what this package needs: trip participants, realtime fan-out, a push sender
//   - store.go     queries (messages, ratings, devices)
//   - chat.go      Send / List / MarkRead
//   - ratings.go   Rate, Summary (average per person)
//   - ratings_admin.go  PersonRatings: both ways, star counts and complaints (staff pages)
//   - devices.go   Register / Unregister push tokens
//   - push.go      the Pusher port's Expo implementation (no key needed) and a log fallback
//   - notify.go    which trip changes become push notifications
package messaging
