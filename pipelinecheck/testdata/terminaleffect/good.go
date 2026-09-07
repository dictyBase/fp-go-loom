// Package terminaleffect is a fixture for the terminal-effect rules.
// good.go converges both branches on a description value and executes
// the effect once, after recovery.
package terminaleffect

import (
	E "github.com/IBM/fp-go/v2/either"
	F "github.com/IBM/fp-go/v2/function"
	IOE "github.com/IBM/fp-go/v2/ioeither"
)

type message struct {
	Topic string
	Body  []byte
}

func encodeOK(s state) E.Either[error, message] { return E.Of[error](message{}) }
func encodeErr(err error) IOE.IOEither[error, message] {
	return IOE.Of[error](message{})
}

func send(msg message) IOE.IOEither[error, int] {
	return IOE.Of[error](len(msg.Body))
}

type state struct{ ID string }

func validate(s state) IOE.IOEither[error, state] { return IOE.Of[error](s) }

// handle recovers before the terminal step, so send runs once.
func handle(s state) IOE.IOEither[error, int] {
	return F.Pipe4(
		IOE.Of[error](s),
		IOE.Chain(validate),
		IOE.ChainEitherK(encodeOK),
		IOE.OrElse(encodeErr),
		IOE.Chain(send),
	)
}

// foldToDescription folds to a value, not to an effect.
func foldToDescription(res E.Either[error, message]) message {
	return E.Fold(
		func(err error) message { return message{Topic: "dlq"} },
		func(msg message) message { return msg },
	)(res)
}
