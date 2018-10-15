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
func GlobalState(ctx context.Context, p *path.GlobalState, r *path.ResolveConfig) (*api.GlobalState, error) {
	ctx = setupContext(ctx, p.After.Capture, r)

	cmdIdx := p.After.Indices[0]
	allCmds, err := Cmds(ctx, p.After.Capture)
	if err != nil {
		return nil, err
	}

	c, err := capture.Resolve(ctx)
	if err != nil {
		return nil, err
	}

	env := c.Env().InitState().Execute().Build(ctx)
	// defer env.Dispose() // TODO: How do we deal with state lifetime here?
	ctx = executor.PutEnv(ctx, env)

	sd, err := SyncData(ctx, p.After.Capture)
	if err != nil {
		return nil, err
	}
	cmds, err := sync.MutationCmdsFor(ctx, p.After.Capture, sd, allCmds, api.CmdID(cmdIdx), p.After.Indices[1:], false)
	if err != nil {
		return nil, err
	}

	defer analytics.SendTiming("resolve", "global-state")(analytics.Count(len(cmds)))

	errs := env.ExecuteN(ctx, 0, cmds)
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}

	return env.State, nil
}

// State resolves the specific API state at a requested point in a capture.
func State(ctx context.Context, p *path.State, r *path.ResolveConfig) (interface{}, error) {
	ctx = capture.Put(ctx, p.After.Capture)
	obj, _, _, err := state(ctx, p, r)
	return obj, err
}

func state(ctx context.Context, p *path.State, r *path.ResolveConfig) (interface{}, path.Node, api.ID, error) {
	cmd, err := Cmd(ctx, p.After, r)
	if err != nil {
		return nil, nil, api.ID{}, err
	}

	a := cmd.API()
	if a == nil {
		return nil, nil, api.ID{}, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable()}
	}

	g, err := GlobalState(ctx, p.After.GlobalStateAfter(), r)
	if err != nil {
		return nil, nil, api.ID{}, err
	}

	state := g.APIs[a.ID()]
	if state == nil {
		return nil, nil, api.ID{}, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable()}
	}

	root, err := state.Root(ctx, p, r)
	if err != nil {
		return nil, nil, api.ID{}, err
	}
	if root == nil {
		return nil, nil, api.ID{}, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable()}
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

	obj, err := Get(ctx, abs.Path(), r)
	if err != nil {
		return nil, nil, api.ID{}, err
	}

	return obj, abs, a.ID(), nil
}

// APIStateAfter returns an absolute path to the API state after c.
func APIStateAfter(ctx context.Context, c *path.Command, a api.ID) path.Node {
	p := &path.GlobalState{After: c}
	return p.Field("APIs").MapIndex(a)
}
