// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

package remote

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"

	"github.com/DataDog/datadog-agent/cmd/agent/api/pb"
	"github.com/DataDog/datadog-agent/cmd/agent/api/response"
	"github.com/DataDog/datadog-agent/pkg/api/util"
	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/status/health"
	"github.com/DataDog/datadog-agent/pkg/tagger/collectors"
	"github.com/DataDog/datadog-agent/pkg/tagger/types"
)

// Tagger holds a connection to a remote tagger, processes incoming events from
// it, and manages the storage of entities to allow querying.
type Tagger struct {
	store *tagStore

	conn   *grpc.ClientConn
	client pb.AgentSecureClient
	stream pb.AgentSecure_TaggerStreamEntitiesClient

	ctx    context.Context
	cancel context.CancelFunc

	health *health.Handle
}

// NewTagger returns an allocated tagger. You still have to run Init()
// once the config package is ready.
func NewTagger() *Tagger {
	return &Tagger{
		store: newTagStore(),
	}
}

// Init initializes the connection to the remote tagger and starts watching for
// events.
func (t *Tagger) Init() error {
	t.health = health.RegisterLiveness("tagger")

	t.ctx, t.cancel = context.WithCancel(context.Background())

	token := util.GetAuthToken()

	conn, err := grpc.DialContext(
		t.ctx,
		fmt.Sprintf(":%v", config.Datadog.GetInt("cmd_port")),

		// NOTE: we're using grpc.WithInsecure because the gRPC server only
		// persists its TLS certs in memory, and we currently have no
		// infrastructure to make them available to clients.
		grpc.WithInsecure(),

		grpc.WithAuthority(fmt.Sprintf("Bearer %s", token)),
	)
	if err != nil {
		return err
	}

	t.client = pb.NewAgentSecureClient(conn)

	go t.run()

	return nil
}

// Stop closes the connection to the remote tagger and stops event collection.
func (t *Tagger) Stop() error {
	t.cancel()

	err := t.conn.Close()
	if err != nil {
		return err
	}

	return nil
}

// Tag returns tags for a given entity at the desired cardinality.
func (t *Tagger) Tag(entityID string, cardinality collectors.TagCardinality) ([]string, error) {
	entity, err := t.store.getEntity(entityID)
	if err != nil {
		return nil, err
	}

	return entity.GetTags(cardinality), nil
}

// Standard returns the standard tags for a given entity.
func (t *Tagger) Standard(entityID string) ([]string, error) {
	entity, err := t.store.getEntity(entityID)
	if err != nil {
		return nil, err
	}

	return entity.StandardTags, nil
}

// List returns all the entities currently stored by the tagger.
func (t *Tagger) List(cardinality collectors.TagCardinality) response.TaggerListResponse {
	entities := t.store.listEntities()
	resp := response.TaggerListResponse{
		Entities: make(map[string]response.TaggerListEntity),
	}

	for _, e := range entities {
		resp.Entities[e.ID] = response.TaggerListEntity{
			Tags: e.GetTags(collectors.HighCardinality),
		}
	}

	return resp
}

// Subscribe is a TODO
func (t *Tagger) Subscribe(cardinality collectors.TagCardinality) chan []types.EntityEvent {
	panic("not implemented") // TODO: Implement
}

// Unsubscribe is a TODO
func (t *Tagger) Unsubscribe(ch chan []types.EntityEvent) {
	panic("not implemented") // TODO: Implement
}

func (t *Tagger) run() {
	err := t.startTaggerStream()
	if err != nil {
		panic(err)
	}

	for {
		select {
		case <-t.health.C:
		case <-t.ctx.Done():
			return
		default:
		}

		response, err := t.stream.Recv()
		if err != nil {
			// Recv() returns io.EOF only when the connection has
			// been intentionally closed by the server. When that
			// happens, we need to establish a new stream. With any
			// other error, including network issues, the stream
			// will remain funcional, as the ClientConn will manage
			// retries accordingly.
			// TODO(juliogreff): verify if that's actually the case :D
			if err == io.EOF {
				err = t.startTaggerStream()
			}

			// TODO(juliogreff): log and continue instead of panic
			panic(fmt.Sprintf("whoops: %s", err))
		}

		err = t.store.processEvent(types.EntityEvent{
			EventType: convertEventType(response.Type),
			Entity: types.Entity{
				ID:                          convertEntityID(response.Entity.Id),
				Hash:                        response.Entity.Hash,
				HighCardinalityTags:         response.Entity.HighCardinalityTags,
				OrchestratorCardinalityTags: response.Entity.OrchestratorCardinalityTags,
				LowCardinalityTags:          response.Entity.LowCardinalityTags,
				StandardTags:                response.Entity.StandardTags,
			},
		})
		if err != nil {
			// TODO(juliogreff): log and continue instead of panic
			panic(fmt.Sprintf("whoops: %s", err))
		}
	}
}

// TODO(juliogreff): should we block until the stream is started, with some
// exponential backoff perhaps?
func (t *Tagger) startTaggerStream() error {
	select {
	case <-t.ctx.Done():
		return nil
	default:
	}

	var err error
	t.stream, err = t.client.TaggerStreamEntities(t.ctx, nil)

	return err
}

// TODO(juliogreff): maybe this should return an error?
func convertEventType(t pb.EventType) types.EventType {
	switch t {
	case pb.EventType_ADDED:
		return types.EventTypeAdded
	case pb.EventType_MODIFIED:
		return types.EventTypeModified
	case pb.EventType_DELETED:
		return types.EventTypeDeleted
	}

	return types.EventTypeAdded
}

func convertEntityID(id *pb.EntityId) string {
	return fmt.Sprintf("%s://%s", id.Prefix, id.Uid)
}
