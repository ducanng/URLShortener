package domain

import "errors"

// ErrNotFound is returned by repository implementations when a requested
// URL key does not exist. Transport adapters map this to codes.NotFound /
// HTTP 404.
var ErrNotFound = errors.New("not found")

// ErrExpired is returned by the service layer when a URL exists in the
// store but its expiry time has already passed. Transport adapters map
// this to codes.FailedPrecondition / HTTP 410 Gone — distinct from
// ErrNotFound so clients can differentiate "never existed" from "expired".
var ErrExpired = errors.New("url expired")
