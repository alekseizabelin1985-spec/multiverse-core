package gatewaytest

import "multiverse-core.io/shared/eventbus/membus"

// WatchBuses hands every bus a FakeGateway makes for itself to seen, until the
// returned function puts the maker back.
func WatchBuses(seen func(*membus.Bus)) (restore func()) {
	made := newBus
	newBus = func() (*membus.Bus, error) {
		bus, err := made()
		if err == nil {
			seen(bus)
		}
		return bus, err
	}
	return func() { newBus = made }
}
