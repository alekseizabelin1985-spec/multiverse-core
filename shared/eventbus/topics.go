package eventbus

// The topics of MVP-1 (ADR-007 p. 4, addendum p. 1-2). The names are constants
// for readability only: which type goes to which topic is decided by the
// contract registry, so there is no routing table here.
//
// scope_management and the phantom topics of the as-is code are gone;
// llm_records, analytics_events and dead_letters are new.
const (
	// TopicPlayerEvents carries player.* and the group movements; retention 30 d.
	TopicPlayerEvents = "player_events"
	// TopicGameEvents carries group.*, round.*, combat.decided, dice.rolled; 30 d.
	TopicGameEvents = "game_events"
	// TopicWorldEvents carries world.*, region.*, npc.*, encounter.*; 30 d.
	TopicWorldEvents = "world_events"
	// TopicSystemEvents carries entity.*, tick.*, agent.*, snapshot.created,
	// content.incident.recorded, config.*; 30 d.
	TopicSystemEvents = "system_events"
	// TopicNarrativeOutput carries narrative.output; 30 d.
	TopicNarrativeOutput = "narrative_output"
	// TopicLLMRecords carries llm.output and llm.output.rejected; 90 d.
	TopicLLMRecords = "llm_records"
	// TopicAnalyticsEvents carries analytics.*; 180 d, not read on replay.
	TopicAnalyticsEvents = "analytics_events"
	// TopicDeadLetters carries the DeadLetter wrapper of anything that could
	// not be handled; 30 d, not read on replay.
	TopicDeadLetters = "dead_letters"
)
