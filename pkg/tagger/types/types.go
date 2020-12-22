package types

import (
	"github.com/DataDog/datadog-agent/pkg/tagger/collectors"
	"github.com/DataDog/datadog-agent/pkg/tagger/utils"
)

// Entity is an entity ID + tags.
type Entity struct {
	ID                          string
	Hash                        string
	HighCardinalityTags         []string
	OrchestratorCardinalityTags []string
	LowCardinalityTags          []string
	StandardTags                []string
}

// GetTags is a TODO
func (e Entity) GetTags(cardinality collectors.TagCardinality) []string {
	tagArrays := make([][]string, 0, 3)
	tagArrays = append(tagArrays, e.LowCardinalityTags)

	switch cardinality {
	case collectors.OrchestratorCardinality:
		tagArrays = append(tagArrays, e.OrchestratorCardinalityTags)
	case collectors.HighCardinality:
		tagArrays = append(tagArrays, e.OrchestratorCardinalityTags)
		tagArrays = append(tagArrays, e.HighCardinalityTags)
	}

	return utils.ConcatenateTags(tagArrays)
}

// EventType is a type of event, triggered when an entity is added, modified or
// deleted.
type EventType int

const (
	// EventTypeAdded means an entity was added.
	EventTypeAdded EventType = iota
	// EventTypeModified means an entity was modified.
	EventTypeModified
	// EventTypeDeleted means an entity was deleted.
	EventTypeDeleted
)

// EntityEvent is an event generated when an entity is added, modified or
// deleted. It contains the event type and the new entity.
type EntityEvent struct {
	EventType EventType
	Entity    Entity
}
