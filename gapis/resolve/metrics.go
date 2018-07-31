// Copyright (C) 2018 Google Inc.
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
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

func Metrics(ctx context.Context, p *path.Metrics) (*api.Metrics, context.Context, error) {
	res := api.Metrics{}
	if p.MemoryBreakdown {
		breakdown, ctx, err := memoryBreakdown(ctx, p.Command)
		if err != nil {
			return nil, ctx, log.Errf(ctx, err, "Failed to get memory breakdown")
		}
		res.MemoryBreakdown = breakdown
	}
	return &res, ctx, nil
}

func memoryBreakdown(ctx context.Context, c *path.Command) (*api.MemoryBreakdown, context.Context, error) {
	cmd, ctx, err := Cmd(ctx, c)
	if err != nil {
		return nil, ctx, err
	}
	a := cmd.API()
	if a == nil {
		return nil, ctx, &service.ErrDataUnavailable{Reason: messages.ErrStateUnavailable(ctx)}
	}

	state, ctx, err := GlobalState(ctx, c.GlobalStateAfter())
	if err != nil {
		return nil, ctx, err
	}
	if ml, ok := a.(api.MemoryBreakdownProvider); ok {
		val, err := ml.MemoryBreakdown(ctx, state)
		return val, ctx, err
	}
	return nil, ctx, fmt.Errorf("Memory breakdown not supported for API %v", a.Name())
}
