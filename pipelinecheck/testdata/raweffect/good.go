package raweffectfixture

import IOE "github.com/IBM/fp-go/v2/ioeither"

type State struct{ Value string }
type Lens struct{}

func (Lens) Set(string) func(State) State { return nil }
func fetchState() (State, error) { return State{}, nil }

func good(l Lens) IOE.IOEither[error, State] {
	return IOE.Map[error](l.Set("value"))(
		IOE.TryCatchError(fetchState),
	)
}
