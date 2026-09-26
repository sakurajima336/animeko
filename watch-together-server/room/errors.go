package room

import "errors"

var (
	// ErrMissingSession 缺少官方下发的 sessionNonce。
	ErrMissingSession = errors.New("MISSING_SESSION")
)
