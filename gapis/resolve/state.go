// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolve

import (
	"context"

	"github.com/google/gapid/core/app/analytics"
	"github.com/google/gapid/gapil/executor"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

// GlobalState resolves the global *api.GlobalState at a requested point in a
// capture.
func GlobalState(ctx context.Context, p *path.GlobalState) (*api.GlobalState, context.Context, error) {
	ctx = capture.Put(ctx, p.After.Capture)
	cmdIdx := p.After.Indices[0]
	allCmds, ctx, err := Cmds(ctx, p.After.Capture)
	if err != nil {
		return nil, ctx, err
	}

	c, err := capture.Resolve(ctx)
	if err != nil {
		return nil, ctx, err
	}

	env := executor.NewEnv(ctx, c, executor.Config{Execute: true})
	ctx = executor.PutEnv(ctx, env)

	sd, err := SyncData(ctx, p.After.Capture)
	if err != nil {
		return nil, ctx, err
	}
	cmds, err := sync.MutationCmdsFor(ctx, p.After.Capture, sd, allCmds, api.CmdID(cmdIdx), p.After.Indices[1:], false)
	if err != nil {
		return nil, ctx, err
	}

	defer analytics.SendTiming("resolve", "global-state")(analytics.Count(len(cmds)))

	err = api.ForeachCmd(ctx, cmds, func(ctx context.Context, id api.CmdID, cmd api.Cmd) error {
		env.Execute(ctx, cmd, id)
		return nil
	})
	if err != nil {
		return nil, ctx, err
	}

	return env.State, ctx, nil
}

// State resolves the specific API state at a requested point in a capture.
func State(ctx context.Context, p *path.State) (interface{}, context.Context, error) {
	ctx = capture.Put(ctx, p.After.Capture)
	obj, ctx, _, _, err := state(ctx, p)
	return obj, ctx, err
}

func state(ctx context.Context, p *path.State) (interface{}, context.Context, path.Node, api.ID, error) {
	cmd, ctx, err := Cmd(ctx, p.After)
	if err != nil {
		return nil, ctx, nil, api.ID{}, err
	}

	a := cmd.API()
	if a == nil {
		return nil, ctx, nil, api.ID{}, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable(ctx)}
	}

	g, ctx, err := GlobalState(ctx, p.After.GlobalStateAfter())
	if err != nil {
		return nil, ctx, nil, api.ID{}, err
	}

	state := g.APIs[a.ID()]
	if state == nil {
		return nil, ctx, nil, api.ID{}, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable(ctx)}
	}

	root, err := state.Root(ctx, p)
	if err != nil {
		return nil, ctx, nil, api.ID{}, err
	}
	if root == nil {
		return nil, ctx, nil, api.ID{}, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable(ctx)}
	}

	// Transform the State path node to a GlobalState node to prevent the
	// object load recursing back into this function.
	abs := path.Transform(root, func(n path.Node) path.Node {
		switch n := n.(type) {
		case *path.State:
			return APIStateAfter(ctx, p.After, a.ID())
		default:
			return n
		}
	})

	obj, err := Get(ctx, abs.Path())
	if err != nil {
		return nil, ctx, nil, api.ID{}, err
	}

	return obj, ctx, abs, a.ID(), nil
}

// APIStateAfter returns an absolute path to the API state after c.
func APIStateAfter(ctx context.Context, c *path.Command, a api.ID) path.Node {
	p := &path.GlobalState{After: c}
	return p.Field("APIs").MapIndex(ctx, a)
}
