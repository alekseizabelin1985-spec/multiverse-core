package api

import "encoding/json"

// The spec declares these arrays required and not nullable, while
// encoding/json writes a nil slice as null. A handler that answers
// AckResponse{Acked: n} would break the contract for every validator but the Go
// client, so the types normalise nil to [] themselves instead of trusting each
// constructor. The receivers are values, so a pointer and a value encode alike.
// openapi_test.go encodes the zero value of every DTO and fails on a null
// required array, which also guards fields added later.

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// MarshalJSON encodes worlds as [] when there are none.
func (v WorldsResponse) MarshalJSON() ([]byte, error) {
	type wire WorldsResponse
	w := wire(v)
	w.Worlds = nonNil(w.Worlds)
	return json.Marshal(w)
}

// MarshalJSON encodes regions as [] when there are none.
func (v WorldSummary) MarshalJSON() ([]byte, error) {
	type wire WorldSummary
	w := wire(v)
	w.Regions = nonNil(w.Regions)
	return json.Marshal(w)
}

// MarshalJSON encodes members as [] when there are none.
func (v GroupView) MarshalJSON() ([]byte, error) {
	type wire GroupView
	w := wire(v)
	w.Members = nonNil(w.Members)
	return json.Marshal(w)
}

// MarshalJSON encodes npcs as [] when there are none.
func (v EncounterView) MarshalJSON() ([]byte, error) {
	type wire EncounterView
	w := wire(v)
	w.NPCs = nonNil(w.NPCs)
	return json.Marshal(w)
}

// MarshalJSON encodes deliveries as [] when nothing arrived within wait_ms.
func (v DeliveriesResponse) MarshalJSON() ([]byte, error) {
	type wire DeliveriesResponse
	w := wire(v)
	w.Deliveries = nonNil(w.Deliveries)
	return json.Marshal(w)
}

// MarshalJSON encodes ids as [] when there are none; the gateway then answers
// 400 invalid_request (minItems 1) instead of failing to decode null.
func (v AckRequest) MarshalJSON() ([]byte, error) {
	type wire AckRequest
	w := wire(v)
	w.IDs = nonNil(w.IDs)
	return json.Marshal(w)
}

// MarshalJSON encodes unknown as [] when every id was known.
func (v AckResponse) MarshalJSON() ([]byte, error) {
	type wire AckResponse
	w := wire(v)
	w.Unknown = nonNil(w.Unknown)
	return json.Marshal(w)
}

// MarshalJSON encodes sessions as [] when none is active.
func (v AdminSessions) MarshalJSON() ([]byte, error) {
	type wire AdminSessions
	w := wire(v)
	w.Sessions = nonNil(w.Sessions)
	return json.Marshal(w)
}
