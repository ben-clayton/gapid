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
	"fmt"

	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/math/u64"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/api/sync"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

// Commands resolves and returns the command list from the path p.
func Commands(ctx context.Context, p *path.Commands) (*service.Commands, context.Context, error) {
	c, err := capture.ResolveFromPath(ctx, p.Capture)
	if err != nil {
		return nil, ctx, err
	}
	cmdIdxFrom, cmdIdxTo := p.From[0], p.To[0]
	if len(p.From) > 1 || len(p.To) > 1 {
		return nil, ctx, fmt.Errorf("Subcommands currently not supported for Commands") // TODO: Subcommands
	}
	count := uint64(len(c.Commands))
	if count == 0 {
		return nil, ctx, fmt.Errorf("No commands in capture")
	}
	cmdIdxFrom = u64.Min(cmdIdxFrom, count-1)
	cmdIdxTo = u64.Min(cmdIdxTo, count-1)
	if cmdIdxFrom > cmdIdxTo {
		cmdIdxFrom, cmdIdxTo = cmdIdxTo, cmdIdxFrom
	}
	count = cmdIdxTo - cmdIdxFrom
	paths := make([]*path.Command, count)
	for i := uint64(0); i < count; i++ {
		paths[i] = p.Capture.Command(i)
	}
	return &service.Commands{List: paths}, ctx, nil
}

// Cmds resolves and returns the command list from the path p.
func Cmds(ctx context.Context, p *path.Capture) ([]api.Cmd, context.Context, error) {
	c, err := capture.ResolveFromPath(ctx, p)
	if err != nil {
		return nil, ctx, err
	}
	return c.Commands, ctx, nil
}

// NCmds resolves and returns the command list from the path p, ensuring
// that the number of commands is at least N.
func NCmds(ctx context.Context, p *path.Capture, n uint64) ([]api.Cmd, context.Context, error) {
	list, ctx, err := Cmds(ctx, p)
	if err != nil {
		return nil, ctx, err
	}
	if count := uint64(len(list)); n > count {
		return nil, ctx, errPathOOB(ctx, n-1, "Index", 0, count-1, p.Command(n-1))
	}
	return list, ctx, nil
}

// Cmd resolves and returns the command from the path p.
func Cmd(ctx context.Context, p *path.Command) (api.Cmd, context.Context, error) {
	cmdIdx := p.Indices[0]
	if len(p.Indices) > 1 {
		snc, err := SyncData(ctx, p.Capture)
		if err != nil {
			return nil, ctx, err
		}

		sg, ok := snc.SubcommandReferences[api.CmdID(cmdIdx)]
		if !ok {
			return nil, ctx, log.Errf(ctx, nil, "Could not find any subcommands on %v", cmdIdx)
		}

		idx := append(api.SubCmdIdx{}, p.Indices[1:]...)
		found := false
		for _, v := range sg {
			if v.Index.Equals(idx) {
				found = true
				cmdIdx = uint64(v.GeneratingCmd)
				if cmdIdx == uint64(api.CmdNoID) {
					capture, err := capture.ResolveFromPath(ctx, p.Capture)
					if err != nil {
						return nil, ctx, err
					}

					for _, api := range capture.APIs {
						if snc, ok := api.(sync.SynchronizedAPI); ok {
							a, err := snc.RecoverMidExecutionCommand(ctx, p.Capture, v.MidExecutionCommandData)
							if err != nil {
								if _, ok := err.(sync.NoMECSubcommandsError); !ok {
									return nil, ctx, err
								}
							} else {
								return a, ctx, nil
							}
						}
					}
					cmdIdx = 0
				}
				break
			}
		}
		if !found {
			return nil, ctx, &service.ErrDataUnavailable{Reason: messages.ErrMessage(ctx, "Not a valid subcommand")}
		}
	}
	cmds, ctx, err := NCmds(ctx, p.Capture, cmdIdx+1)
	if err != nil {
		return nil, ctx, err
	}
	return cmds[cmdIdx], ctx, nil
}

// Parameter resolves and returns the parameter from the path p.
func Parameter(ctx context.Context, p *path.Parameter) (interface{}, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}
	cmd := obj.(api.Cmd)
	param, err := api.GetParameter(cmd, p.Name)
	switch err {
	case nil:
		return param, ctx, nil
	case api.ErrParameterNotFound:
		return nil, ctx, &service.ErrInvalidPath{
			Reason: messages.ErrParameterDoesNotExist(ctx, cmd.CmdName(), p.Name),
			Path:   p.Path(),
		}
	default:
		return nil, ctx, err
	}
}

// Result resolves and returns the command's result from the path p.
func Result(ctx context.Context, p *path.Result) (interface{}, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}
	cmd := obj.(api.Cmd)
	param, err := api.GetResult(cmd)
	switch err {
	case nil:
		return param, ctx, nil
	case api.ErrResultNotFound:
		return nil, ctx, &service.ErrInvalidPath{
			Reason: messages.ErrResultDoesNotExist(ctx, cmd.CmdName()),
			Path:   p.Path(),
		}
	default:
		return nil, ctx, err
	}
}
