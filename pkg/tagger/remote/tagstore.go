// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

package remote

import "github.com/DataDog/datadog-agent/pkg/tagger/types"

type tagStore struct {
	store map[string]types.Entity
}

func newTagStore() *tagStore {
	return &tagStore{
		store: make(map[string]types.Entity),
	}
}

func (s *tagStore) processEvent(event types.EntityEvent) error {
	switch event.EventType {
	case types.EventTypeAdded, types.EventTypeModified:
		s.store[event.Entity.ID] = event.Entity
	case types.EventTypeDeleted:
		delete(s.store, event.Entity.ID)
	}

	return nil
}

func (s *tagStore) getEntity(entityID string) (types.Entity, error) {
	return s.store[entityID], nil
}

func (s *tagStore) listEntities() []types.Entity {
	entities := make([]types.Entity, 0, len(s.store))

	for _, e := range s.store {
		entities = append(entities, e)
	}

	return entities
}
