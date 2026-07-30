package crossfile

import IO "github.com/IBM/fp-go/v2/io"

func bare(n int) IO.IO[int] { return IO.Printf[int](bareFmt) }
func verb(n int) IO.IO[int] { return IO.Printf[int](verbFmt) }
func both(n int) IO.IO[int] {
	return IO.Printf[int](prefix + suffix)
}
