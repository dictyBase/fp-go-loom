// Package crossfile is a SafePrintf fixture proving cross-file
// package-local constant resolution. defs.go declares the constants;
// use.go calls IO.Printf with them.
package crossfile
