// Package store is the bounded ephemeral message ring (wait, wipe, generation).
// Production code must not Dial. Insert rejects with store_full; it does not
// close sockets (C18).
package store
