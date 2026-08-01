// Package fs provides predicates over io/fs.FileInfo, derived from
// the FileInfo interface methods.
package fs

import (
	iofs "io/fs"

	F "github.com/IBM/fp-go/v2/function"
	P "github.com/IBM/fp-go/v2/predicate"
)

var (
	// IsDirInfo is a Predicate[iofs.FileInfo] that is true when the
	// FileInfo reports a directory. Compose it with P.ContraMap and
	// any extractor to lift it onto a state type.
	IsDirInfo = func(fi iofs.FileInfo) bool { return fi.IsDir() }

	// IsRegularInfo is a Predicate[iofs.FileInfo] that is true when
	// the FileInfo reports a regular file. Compose it with P.ContraMap
	// and any extractor to lift it onto a state type.
	IsRegularInfo = func(fi iofs.FileInfo) bool { return fi.Mode().IsRegular() }
)

// IsDir returns a Predicate[S] that is true when the FileInfo
// extracted from a value of type S reports a directory. The
// extractor must return a non-nil FileInfo; pass a lens getter for
// callers that own a lens.
func IsDir[S any](get func(S) iofs.FileInfo) P.Predicate[S] {
	return F.Pipe1(IsDirInfo, P.ContraMap(get))
}

// IsRegular returns a Predicate[S] that is true when the FileInfo
// extracted from a value of type S reports a regular file. The
// extractor must return a non-nil FileInfo; pass a lens getter for
// callers that own a lens.
func IsRegular[S any](get func(S) iofs.FileInfo) P.Predicate[S] {
	return F.Pipe1(IsRegularInfo, P.ContraMap(get))
}
