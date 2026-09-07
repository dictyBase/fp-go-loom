package terminaleffect

import (
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

// encodeAndSend both encodes and executes the effect, so recovery
// placed after it re-runs the send.
func encodeAndSend(s state) IOE.IOEither[error, int] {
	return F.Pipe2(
		IOE.Of[error](s),
		IOE.ChainEitherK(encodeOK),
		IOE.Chain(send),
	)
}

func recoverAfterSend(s state) IOE.IOEither[error, int] {
	return F.Pipe3(
		IOE.Of[error](s),
		IOE.Chain(validate),
		IOE.Chain(encodeAndSend),
		IOE.OrElse(recoverSend),
	)
}

func recoverSend(err error) IOE.IOEither[error, int] {
	return IOE.Of[error](0)
}
