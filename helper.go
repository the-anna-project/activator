package activator

import (
	"github.com/the-anna-project/event"
)

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, e := range a {
		if e != b[i] {
			return false
		}
	}

	return true
}

func removeSignals(queue, toRemove []event.Signal) []event.Signal {
	var newQueue []event.Signal

	for _, q := range queue {
		for _, t := range toRemove {
			if q.ID() == t.ID() {
				continue
			}

			newQueue = append(newQueue, q)
		}
	}

	return newQueue
}
