---
name: guardian-test-monitor
version: "1.0"
description: A guardian monitor, a level reserved for later epics (rule 14)
level: monitor
role: guardian-monitor
scope_binding: { type: world, pattern: "world:*" }
parent: { name: global-test-world }
trigger: { type: event, event_name: "world.*" }
constraints: { max_instances: 1 }
allowed_event_types: []
owned_entity_types: []
locale: ru
---
