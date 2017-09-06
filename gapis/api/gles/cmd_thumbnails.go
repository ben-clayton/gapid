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

package gles

import (
	"context"

	"github.com/google/gapid/core/image"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/gapis/resolve"
	"github.com/google/gapid/gapis/service/path"
)

func texThumbnailByTarget(ctx context.Context, p *path.Thumbnail, thread uint64, target GLenum, level GLint) (*image.Info, error) {
	pc := p.GetCommand()
	if pc == nil {
		log.W(ctx, "Path does not have a command: %v", p)
		return nil, nil
	}
	s, err := resolve.GlobalState(ctx, pc.StateAfter())
	if err != nil {
		log.W(ctx, "Couldn't get state: %v", err)
		return nil, err
	}
	c := GetContext(s, thread)
	if c == nil {
		log.W(ctx, "Couldn't get context")
		return nil, nil
	}
	tex := getBoundTexture(target, s, thread)
	if tex == nil {
		log.W(ctx, "Couldn't get texture")
		return nil, nil
	}
	data, err := tex.ResourceData(ctx, s)
	if err != nil {
		log.W(ctx, "Couldn't get data: %v", err)
		return nil, err
	}
	return data.Thumbnail(ctx, p) // TODO: Display the specified level
}

var _ = []path.Thumbnailer{
	(*GlTexImage2D)(nil),
	(*GlBindTexture)(nil),
}

func (c *GlTexImage2D) Thumbnail(ctx context.Context, p *path.Thumbnail) (*image.Info, error) {
	return texThumbnailByTarget(ctx, p, c.thread, c.Target, c.Level)
}

func (c *GlBindTexture) Thumbnail(ctx context.Context, p *path.Thumbnail) (*image.Info, error) {
	return texThumbnailByTarget(ctx, p, c.thread, c.Target, -1)
}
