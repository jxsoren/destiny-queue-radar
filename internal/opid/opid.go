// Package opid carries an operation's request ID through a context, so every log line an
// operation writes (including its Bungie calls) can be found by that ID.
package opid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type key struct{}

// New returns a random 16-character hex ID.
func New() string {
	var b [8]byte
	rand.Read(b[:]) // never fails (crypto/rand docs)
	return hex.EncodeToString(b[:])
}

// With returns ctx carrying id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// From returns the ID carried by ctx, or "" if there is none.
func From(ctx context.Context) string {
	id, _ := ctx.Value(key{}).(string)
	return id
}
