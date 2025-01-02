package fspath

// Local is a machine-dependent path representation. It is the format expected by functions in the
// path/filepath module.
type Local = string

// POSIX is a forward-slash delimited path representation. It is the format expected by functions in
// the path module.
type POSIX = string
