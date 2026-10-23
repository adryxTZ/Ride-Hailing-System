// Package realtime is the websocket hub: one socket per app session at /v1/ws.
//
//	frame.go   the wire envelope and frame types
//	hub.go     local connections + Redis pub/sub fan-out (Publish works from any process)
//	conn.go    the HTTP handler: first-frame auth, heartbeat, write pump
//
// Protocol: the client opens the socket and sends {"type":"auth","token":"<access token>"}
// within 10 s (the token never appears in a URL). The server answers {"type":"ready"}.
// Afterwards the client may send {"type":"ping"}; the server sends frames addressed
// to the signed-in user: trip.updated, offer.new, offer.closed, driver.location.
// Delivery is at-most-once; clients re-read state over HTTP after reconnecting.
package realtime
