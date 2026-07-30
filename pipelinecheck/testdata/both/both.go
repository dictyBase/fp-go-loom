// Package both contains one TryCatch violation and one verb-less
// Printf violation, used to prove the public APIs are independent:
// Check finds neither, CheckNoIfErrInTryCatch finds one, CheckSafePrintf
// finds one.
package both

import (
	IO "github.com/IBM/fp-go/v2/io"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

type T struct{}

func get() (*T, error) { return nil, nil }

func badTryCatch() IOE.IOEither[error, *T] {
	return IOE.TryCatchError(func() (*T, error) {
		x, err := get()
		if err != nil {
			return nil, err
		}
		return x, nil
	})
}

func badPrintf(n int) IO.IO[int] {
	return IO.Printf[int]("plain")
}
