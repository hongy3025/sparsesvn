package executor

import "errors"

var (
	ErrURLMismatch    = errors.New("url mismatch with state file")
	ErrURLRequired    = errors.New("url required: provide in config or --url flag")
	ErrConfigInvalid  = errors.New("config invalid")
	ErrSvnFailed      = errors.New("svn command failed")
	ErrStateCorrupt   = errors.New("state file corrupt; delete it to trigger full rebuild")
	ErrStateFutureVer = errors.New("state file version newer than supported; please upgrade sparsesvn")
)
