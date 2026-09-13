// Package actions accepts the actions of players: it validates an action
// against the projection of the world, keeps the answer under its action_key,
// limits how often a player acts, passes the text of a player through the
// input filter and publishes the player.* event with the proposals that go
// with it (component gateway-and-bot.md §5.4, §5.5, §7.2; C-04, C-08).
package actions

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/entity"
)

// The event types the actions become (api-contracts.md §2.3.1, C-02).
const (
	TypeEnteredRegion  = "player.entered_region"
	TypeLeftRegion     = "player.left_region"
	TypeLooked         = "player.looked"
	TypeAttacked       = "player.attacked"
	TypeFleeAttempted  = "player.flee_attempted"
	TypeRested         = "player.rested"
	TypeSaid           = "player.said"
	TypeDefended       = "player.defended"
	TypeUpdateProposed = "entity.update.proposed"
	TypeGMCreated      = "gm.created"
)

// MaxTextRunes bounds the text of say after trimming (api-contracts.md §1.4,
// the maxLength of player.said.text).
const MaxTextRunes = 500

// MaxActionKey bounds action_key (api-contracts.md §1.1).
const MaxActionKey = 64

// Target is what the target of an action names.
type Target int

const (
	TargetNone Target = iota
	TargetRegion
	TargetNPC
	TargetGroup
)

// Encounter is where an action may be taken relative to an encounter.
type Encounter int

const (
	// Anywhere: in an encounter or outside one.
	Anywhere Encounter = iota
	// Outside: in_encounter when the character is in one.
	Outside
	// Inside: not_in_encounter when the character is in none.
	Inside
)

// Rule is one row of the action dictionary of api-contracts.md §1.4.
type Rule struct {
	Target    Target
	Text      bool
	Encounter Encounter
	// InRegion: not_in_region when the character stands outside every region.
	InRegion bool
	// Group marks the group.* actions, whose rules and effects come with the
	// groups of increment I2 (T-352).
	Group bool
	// Event is the type the action is published as.
	Event string
}

// Rules is the action dictionary: a direct encoding of api-contracts.md §1.4.
// The preconditions of a group (a group move by its leader, the round of a
// group scope) are not here: groups do not exist before T-352.
var Rules = map[string]Rule{
	api.ActionEnter:       {Target: TargetRegion, Encounter: Outside, Event: TypeEnteredRegion},
	api.ActionLeave:       {Encounter: Outside, InRegion: true, Event: TypeLeftRegion},
	api.ActionLook:        {Event: TypeLooked},
	api.ActionAttack:      {Target: TargetNPC, Encounter: Inside, Event: TypeAttacked},
	api.ActionFlee:        {Encounter: Inside, Event: TypeFleeAttempted},
	api.ActionRest:        {Encounter: Outside, Event: TypeRested},
	api.ActionSay:         {Text: true, Event: TypeSaid},
	api.ActionDefend:      {Encounter: Inside, Event: TypeDefended},
	api.ActionGroupCreate: {Group: true},
	api.ActionGroupJoin:   {Target: TargetGroup, Encounter: Outside, Group: true},
	api.ActionGroupLeave:  {Group: true},
}

// Command is an action as the HTTP handler received it.
type Command struct {
	PlayerID  string
	ActionKey string
	Type      string
	// Target is empty when the request had none.
	Target string
	// Text is nil when the request had none.
	Text      *string
	ActorKind string
}

// Projection is what validation reads of the read model.
type Projection interface {
	Character(id string) (readmodel.CharacterState, bool)
	NPC(id string) (readmodel.NPC, bool)
	Region(worldID, regionID string) (readmodel.Region, bool)
	EncounterOf(playerID string) (readmodel.Encounter, bool)
	// Version finds an entity of any type and any world.
	Version(id string) (int64, bool)
}

// Validated is an action that passed every precondition, with what the checks
// found on the way: the event is built from it without reading the projection
// again.
type Validated struct {
	Command   Command
	Rule      Rule
	Character readmodel.CharacterState
	// Region is the target of enter, and the region the character stands in
	// for leave and look; nil outside every region.
	Region *readmodel.Region
	// NPC is the target of attack.
	NPC *readmodel.NPC
	// Encounter is the open encounter of the character, when there is one.
	Encounter *readmodel.Encounter
	// Text is the trimmed text of say.
	Text string
}

// Validator checks actions against a projection.
type Validator struct {
	Model Projection
	// Grace is how long an active encounter may run without a task agent
	// before its actions answer encounter_unavailable (MV_GATEWAY_ENCOUNTER_GRACE).
	Grace time.Duration
}

// Validate runs the checks of component §5.4 in their fixed order and returns
// the first that fails:
//
//  1. player_not_found, character_dead, unknown_action, then the structure of
//     the request — invalid_request, text_invalid, and unknown_target for a
//     target no entity has;
//  2. the group and its leader — not before the groups of T-352;
//  3. the encounter — in_encounter, not_in_encounter, target_dead,
//     encounter_unavailable;
//  4. the region — not_in_region, and unknown_target for a region of another
//     world or an NPC outside the encounter;
//  5. the round of a group scope — not before T-350.
//
// A group.* action that passes step 1 answers not_implemented until T-352.
func (v Validator) Validate(c Command, now time.Time) (Validated, *api.Error) {
	ch, ok := v.Model.Character(c.PlayerID)
	if !ok {
		return Validated{}, api.NewError(api.CodePlayerNotFound, nil)
	}
	if entity.IsTerminalStatus(ch.Status) {
		return Validated{}, api.NewError(api.CodeCharacterDead, nil)
	}
	rule, ok := Rules[c.Type]
	if !ok {
		return Validated{}, api.NewError(api.CodeUnknownAction, nil)
	}
	out := Validated{Command: c, Rule: rule, Character: ch}
	if e := v.structure(&out); e != nil {
		return Validated{}, e
	}
	if rule.Group {
		return Validated{}, api.NewError(api.CodeNotImplemented, map[string]any{"type": c.Type})
	}
	if e := v.encounter(&out, now); e != nil {
		return Validated{}, e
	}
	if e := v.region(&out); e != nil {
		return Validated{}, e
	}
	return out, nil
}

func (v Validator) structure(out *Validated) *api.Error {
	c, rule := out.Command, out.Rule
	switch {
	case rule.Target == TargetNone && c.Target != "":
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "target"})
	case rule.Target != TargetNone && c.Target == "":
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "target"})
	case !rule.Text && c.Text != nil:
		return api.NewError(api.CodeInvalidRequest, map[string]any{"field": "text"})
	}
	if rule.Text {
		text, ok := CheckText(c.Text)
		if !ok {
			return api.NewError(api.CodeTextInvalid, nil)
		}
		out.Text = text
	}
	if c.Target != "" {
		if _, known := v.Model.Version(c.Target); !known {
			return api.NewError(api.CodeUnknownTarget, nil)
		}
	}
	return nil
}

// CheckText trims the text of say and reports whether it is 1 to MaxTextRunes
// characters of valid UTF-8 without control characters (NFR-043).
func CheckText(text *string) (string, bool) {
	if text == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(*text)
	if !utf8.ValidString(trimmed) {
		return "", false
	}
	n := utf8.RuneCountInString(trimmed)
	if n < 1 || n > MaxTextRunes {
		return "", false
	}
	if strings.IndexFunc(trimmed, unicode.IsControl) >= 0 {
		return "", false
	}
	return trimmed, true
}

func (v Validator) encounter(out *Validated, now time.Time) *api.Error {
	enc, in := v.Model.EncounterOf(out.Character.ID)
	if in {
		out.Encounter = &enc
	}
	switch out.Rule.Encounter {
	case Outside:
		if in {
			return api.NewError(api.CodeInEncounter, nil)
		}
		return nil
	case Inside:
		if !in {
			return api.NewError(api.CodeNotInEncounter, nil)
		}
	default:
		return nil
	}
	if out.Rule.Target == TargetNPC {
		if npc, ok := v.Model.NPC(out.Command.Target); ok {
			out.NPC = &npc
			if entity.IsTerminalStatus(npc.Status) {
				return api.NewError(api.CodeTargetDead, nil)
			}
		}
	}
	// An encounter only announced has no entity yet and so no agent to wait
	// for; its entity follows the announcement (C-04 v1.3). An entity without
	// the time of its creation (an old snapshot) gives the grace nothing to
	// count from, and the agent is not presumed missing.
	if enc.State == entity.EncounterStateActive && enc.TaskAgentID == "" && !enc.CreatedAt.IsZero() &&
		now.Sub(enc.CreatedAt) > v.Grace {
		return api.NewError(api.CodeEncounterUnavailable, nil)
	}
	return nil
}

func (v Validator) region(out *Validated) *api.Error {
	ch := out.Character
	switch out.Rule.Target {
	case TargetRegion:
		region, ok := v.Model.Region(ch.WorldID, out.Command.Target)
		if !ok {
			return api.NewError(api.CodeUnknownTarget, nil)
		}
		out.Region = &region
		return nil
	case TargetNPC:
		if out.NPC == nil || out.Encounter == nil || !slices.Contains(out.Encounter.NPCIDs, out.NPC.ID) {
			return api.NewError(api.CodeUnknownTarget, nil)
		}
	}
	if region, ok := v.Model.Region(ch.WorldID, ch.Position); ok {
		out.Region = &region
	} else if out.Rule.InRegion {
		return api.NewError(api.CodeNotInRegion, nil)
	}
	return nil
}
