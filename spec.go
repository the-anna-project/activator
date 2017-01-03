package activator

import (
	"github.com/the-anna-project/context"
	"github.com/the-anna-project/event"
)

// Service represents an management layer to organize CLG activation rules. The
// activator obtains signals for every single requested CLG within every
// possible CLG tree.
type Service interface {
	// Activate applies different activation algorithms to check whether the
	// requested CLG should be activated. Activation algorithms are provided by
	// the following functions.
	//
	//     Service.WithConfiguration
	//     Service.WithInputTypes
	//
	// The provided context is scoped to the internals of the execution process of
	// the current CLG. It does not have anything to do with an event context.
	//
	// The provided signal is the current signal being used to activate the
	// current CLG.
	Activate(ctx context.Context, signal event.Signal) (event.Signal, error)
	// Boot initializes and starts the whole service like booting a machine. The
	// call to Boot blocks until the service is completely initialized, so you
	// might want to call it in a separate goroutine.
	Boot()
	// WithConfiguration compares the given queue against the stored configuration
	// of the requested CLG. This configuration is a combination of behaviour IDs
	// that are known to be successful in combination. In case the given signal
	// queue contains signals sent by the CLGs listed in the stored configuration,
	// the interface of the requested CLG is fulfilled. Then a new signal is
	// created by merging the matching signals of the given queue to one new
	// signal. In case no activation configuration of the requested CLG is stored,
	// or no match using the stored configuration associated with the requested
	// CLG can be found, an error is returned.
	//
	// The provided context is scoped to the internals of the execution process of
	// the current CLG. It does not have anything to do with an event context.
	//
	// The provided signal is the current signal being used to activate the
	// current CLG.
	WithConfiguration(ctx context.Context, signal event.Signal, queue []event.Signal) (event.Signal, []event.Signal, error)
	// WithInputTypes uses the given queue to find a combination of arguments
	// carried by the queue's signals, which fulfills the interface of the
	// requested CLG. The creation process of Permute may be random or biased in
	// some way. In case some combination of arguments fulfills the interface of
	// the requested CLG, this found combination is stored as activation
	// configuration for the requested CLG. The configuration identifier is the
	// CLG's behaviour ID. In case no match using the permuted signals can be
	// found, an error is returned. In case a combination could be found, the
	// arguments of the found combination are put together in a new signal.
	// Further the signals which provided the arguments for the successful
	// combination are removed from the queue. This filtered list is also
	// returned.
	//
	// The provided context is scoped to the internals of the execution process of
	// the current CLG. It does not have anything to do with an event context.
	//
	// The provided signal is the current signal being used to activate the
	// current CLG.
	WithInputTypes(ctx context.Context, signal event.Signal, queue []event.Signal) (event.Signal, []event.Signal, error)
	// Shutdown ends all processes of the service like shutting down a machine.
	// The call to Shutdown blocks until the service is completely shut down, so
	// you might want to call it in a separate goroutine.
	Shutdown()
}
