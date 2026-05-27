module multiverse-core.io/shared/agent/tools

go 1.24.0

require (
	multiverse-core.io/shared/agent v0.0.0
	multiverse-core.io/shared/eventbus v0.0.0
)

replace (
	multiverse-core.io/shared/agent => ../
	multiverse-core.io/shared/eventbus => ../../eventbus
)
