package contracts

// Types of EPIC-004. The owner adds the line of its type here, reviewed by the
// system architect (contracts.md §16 p. 8); the order of the lists is in
// registry.go.
var gatewayDefinitions = []Spec{
	// --- block "а": player actions, groups, rounds (owner EPIC-004) ---
	playerEvent("player.entered_region", []string{SourceSwarm, SourceMemory, SourceLLM}),
	playerEvent("player.left_region", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.looked", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.attacked", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.flee_attempted", []string{SourceSwarm}),
	playerEvent("player.rested", []string{SourceSwarm}),
	playerEvent("player.said", []string{SourceSwarm, SourceMemory}),
	playerEvent("player.defended", []string{SourceSwarm, SourceMemory}),

	gameEvent("group.created", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.joined", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.left", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.leader_changed", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	gameEvent("group.disbanded", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm, SourceMemory}),
	// A group moves on player_events: it is the only wire format of a group
	// movement, so it travels with the movements of solo players (C-04).
	playerEvent("group.entered_region", []string{SourceSwarm, SourceMemory}),
	playerEvent("group.left_region", []string{SourceSwarm, SourceMemory}),

	gameEvent("round.opened", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm}),
	gameEvent("round.closed", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceSwarm}),
}

// gatewayAnalyticsDefinitions are the analytics of the gateway (C-10): it
// measures the session and the turn, and the operator CLI of EPIC-005 reports
// on them. Nothing here is read during a replay.
var gatewayAnalyticsDefinitions = []Spec{
	analyticsEvent("analytics.session.started", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceMvctl}),
	analyticsEvent("analytics.session.ended", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceMvctl}),
	analyticsEvent("analytics.turn.completed", OwnerGateway,
		[]string{SourceGateway, SourceTestkitGateway}, []string{SourceMvctl}),
}
