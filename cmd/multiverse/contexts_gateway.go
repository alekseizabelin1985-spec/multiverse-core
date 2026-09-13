package main

import (
	"multiverse-core.io/internal/gateway"
	"multiverse-core.io/shared/runtime"
)

// Factory of the context of EPIC-004. The owner changes the value on the right
// and nothing else; the name and the start order stay in contexts.go.
//
// gateway is real since T-303: it is built over the process environment, and
// its variables (MV_GATEWAY_DATA_DIR and the client lists) are read in its
// Start, not here, for the reason newSwarm gives.
var newGatewayContext = func() runtime.Context { return gateway.New(nil) }
