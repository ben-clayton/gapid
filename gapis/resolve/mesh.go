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

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

// Mesh resolves and returns the Mesh from the path p.
func Mesh(ctx context.Context, p *path.Mesh) (*api.Mesh, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}
	mesh, ctx, err := meshFor(ctx, obj, p)
	switch {
	case err != nil:
		return nil, ctx, err
	case mesh != nil:
		return mesh, ctx, nil
	default:
		return nil, ctx, &service.ErrDataUnavailable{Reason: messages.ErrMeshNotAvailable(ctx)}
	}
}

func meshFor(ctx context.Context, o interface{}, p *path.Mesh) (*api.Mesh, context.Context, error) {
	switch o := o.(type) {
	case api.APIObject:
		if a := o.API(); a != nil {
			if mp, ok := a.(api.MeshProvider); ok {
				val, err := mp.Mesh(ctx, o, p)
				return val, ctx, err
			}
		}

	case *service.CommandTreeNode:
		cmds, ctx, err := Cmds(ctx, o.Commands.Capture)
		if err != nil {
			return nil, ctx, err
		}

		if len(o.Commands.From) != len(o.Commands.To) {
			return nil, ctx, log.Errf(ctx, nil, "Subcommand indices must be the same length")
		}

		if len(o.Commands.From) == 1 {
			s, e := o.Commands.From[0], o.Commands.To[0]
			for i := e; int64(i) >= int64(s); i-- {
				p := o.Commands.Capture.Command(i).Mesh(p.Options)
				if mesh, ctx, err := meshFor(ctx, cmds[i], p); mesh != nil || err != nil {
					return mesh, ctx, err
				}
			}
		} else {
			lastSubcommand := len(o.Commands.From) - 1
			for i := 0; i < lastSubcommand; i++ {
				if o.Commands.From[i] != o.Commands.To[i] {
					return nil, ctx, log.Errf(ctx, nil, "Subcommand ranges must be identical everywhere but the last element")
				}
			}

			for i := o.Commands.To[lastSubcommand]; i >= o.Commands.From[lastSubcommand]; i-- {
				cmd := append([]uint64{}, o.Commands.From[1:]...)
				cmd[lastSubcommand-1] = i
				p := o.Commands.Capture.Command(o.Commands.From[0], cmd...).Mesh(p.Options)
				if mesh, ctx, err := meshFor(ctx, cmds[o.Commands.From[0]], p); mesh != nil || err != nil {
					return mesh, ctx, err
				}
			}
		}

		return nil, ctx, &service.ErrDataUnavailable{Reason: messages.ErrNotADrawCall(ctx)}
	}
	return nil, ctx, nil
}
