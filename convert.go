package activator

import (
	"reflect"

	sourceids "github.com/the-anna-project/context/source/ids"
	"github.com/the-anna-project/event"
)

func eventsToSignals(events []event.Event) ([]event.Signal, error) {
	var signals []event.Signal

	for _, e := range events {
		s, err := event.NewSignalFromEvent(e)
		if err != nil {
			return nil, maskAny(err)
		}
		signals = append(signals, s)
	}

	return signals, nil
}

// queueToValues parses a list of signals to permutation values.
func queueToValues(queue []event.Signal) []interface{} {
	var values []interface{}

	for _, s := range queue {
		values = append(values, s)
	}

	return values
}

func signalsToEvents(queue []event.Signal) []event.Event {
	var events []event.Event

	for _, s := range queue {
		events = append(events, s.(event.Event))
	}

	return events
}

// valuesToArgumentTypes parses permutation values to a list of strings
// representing argument types of the given signals. The underlying type of each
// permutation value must be event.Signal. If the underlying type of the given
// values is not event.Signal, an error is returned.
func valuesToArgumentTypes(values []interface{}) ([]string, error) {
	var types []reflect.Type
	{
		for _, v := range values {
			signal, ok := v.(event.Signal)
			if !ok {
				return nil, maskAnyf(invalidExecutionError, "permutation value must be event signal")
			}
			for _, argument := range signal.Arguments() {
				types = append(types, argument.Type())
			}
		}
	}

	var strings []string
	{
		for _, t := range types {
			strings = append(strings, t.String())
		}
	}

	return strings, nil
}

// valuesToQueue parses permutation values to signal events. The underlying type
// of each permutation value must be event.Signal. If the underlying type of the
// given values is not event.Signal, an error is returned.
func valuesToQueue(values []interface{}) ([]event.Signal, error) {
	var queue []event.Signal

	for _, v := range values {
		signal, ok := v.(event.Signal)
		if !ok {
			return nil, maskAnyf(invalidExecutionError, "permutation value must be event signal")
		}
		queue = append(queue, signal)
	}

	return queue, nil
}

// valuesToSourceIDs parses permutation values to a list of strings representing
// source IDs of the given signals. The underlying type of each permutation
// value must be event.Signal. If the underlying type of the given values is not
// event.Signal, an error is returned.
func valuesToSourceIDs(values []interface{}) ([]string, error) {
	var strings []string
	{
		for _, v := range values {
			signal, ok := v.(event.Signal)
			if !ok {
				return nil, maskAnyf(invalidExecutionError, "permutation value must be event signal")
			}
			sourceIDs, ok := sourceids.FromContext(signal.Context())
			if !ok {
				return nil, maskAnyf(invalidContextError, "source ids must not be empty")
			}
			strings = append(strings, sourceIDs...)
		}
	}

	return strings, nil
}
