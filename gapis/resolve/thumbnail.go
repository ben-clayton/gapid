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

	"github.com/google/gapid/core/image"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
)

// Thumbnail resolves and returns the thumbnail from the path p.
func Thumbnail(ctx context.Context, p *path.Thumbnail) (*image.Info, error) {
	img, err := func() (*image.Info, error) {
		switch parent := p.Parent().(type) {
		case *path.Command:
			return CommandThumbnail(ctx, p, parent)
		case *path.CommandTreeNode:
			return CommandTreeNodeThumbnail(ctx, p, parent)
		case *path.ResourceData:
			return ResourceDataThumbnail(ctx, p, parent)
		default:
			return nil, fmt.Errorf("Unexpected Thumbnail parent %T", parent)
		}
	}()

	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, &service.ErrDataUnavailable{Reason: messages.ErrNoTextureData("")}
	}

	if p.DesiredFormat != nil {
		// Convert the image to the desired format.
		if img.Format.Key() != p.DesiredFormat.Key() {
			img, err = img.Convert(ctx, p.DesiredFormat)
			if err != nil {
				return nil, err
			}
		}
	}

	// Image format supports resizing. See if the image should be.
	scaleX, scaleY := float32(1), float32(1)
	if p.DesiredMaxWidth > 0 && img.Width > p.DesiredMaxWidth {
		scaleX = float32(p.DesiredMaxWidth) / float32(img.Width)
	}
	if p.DesiredMaxHeight > 0 && img.Height > p.DesiredMaxHeight {
		scaleY = float32(p.DesiredMaxHeight) / float32(img.Height)
	}
	scale := scaleX // scale := min(scaleX, scaleY)
	if scale > scaleY {
		scale = scaleY
	}

	targetWidth := uint32(float32(img.Width) * scale)
	targetHeight := uint32(float32(img.Height) * scale)

	// Prevent scaling to zero size.
	if targetWidth == 0 {
		targetWidth = 1
	}
	if targetHeight == 0 {
		targetHeight = 1
	}

	if targetWidth == img.Width && targetHeight == img.Height {
		// Image is already at requested target size.
		return img, err
	}

	return img.Resize(ctx, targetWidth, targetHeight, 1)
}

// CommandThumbnail resolves and returns the thumbnail for the framebuffer at p.
func CommandThumbnail(ctx context.Context, pt *path.Thumbnail, pc *path.Command) (*image.Info, error) {
	if cmd, _ := Cmd(ctx, pc); cmd != nil {
		if t, ok := cmd.(path.Thumbnailer); ok {
			pt := pc.Thumbnail(pt.DesiredMaxWidth, pt.DesiredMaxHeight, pt.DesiredFormat)
			return t.Thumbnail(ctx, pt)
		}
	}

	imageInfoPath, err := FramebufferAttachment(ctx,
		nil, // device
		pc,
		api.FramebufferAttachment_Color0,
		&service.RenderSettings{
			MaxWidth:      pt.DesiredMaxWidth,
			MaxHeight:     pt.DesiredMaxHeight,
			WireframeMode: service.WireframeMode_None,
		},
		&service.UsageHints{
			Preview: true,
		},
	)
	if err != nil {
		return nil, err
	}

	var boxedImageInfo interface{}
	if pt.DesiredFormat != nil {
		boxedImageInfo, err = Get(ctx, imageInfoPath.As(pt.DesiredFormat).Path())
	} else {
		boxedImageInfo, err = Get(ctx, imageInfoPath.Path())
	}
	if err != nil {
		return nil, err
	}

	return boxedImageInfo.(*image.Info), nil
}

// CommandTreeNodeThumbnail resolves and returns the thumbnail for the framebuffer at p.
func CommandTreeNodeThumbnail(ctx context.Context, pt *path.Thumbnail, pn *path.CommandTreeNode) (*image.Info, error) {
	boxedCmdTree, err := database.Resolve(ctx, pn.Tree.ID())
	if err != nil {
		return nil, err
	}

	cmdTree := boxedCmdTree.(*commandTree)

	switch item := cmdTree.index(pn.Indices).(type) {
	case api.CmdIDGroup:
		thumbnail := item.Range.Last()
		if userData, ok := item.UserData.(*CommandTreeNodeUserData); ok {
			thumbnail = userData.Thumbnail
		}
		return CommandThumbnail(ctx, pt, cmdTree.path.Capture.Command(uint64(thumbnail)))
	case api.SubCmdIdx:
		return CommandThumbnail(ctx, pt, cmdTree.path.Capture.Command(uint64(item[0]), item[1:]...))
	case api.SubCmdRoot:
		return CommandThumbnail(ctx, pt, cmdTree.path.Capture.Command(uint64(item.Id[0]), item.Id[1:]...))
	default:
		panic(fmt.Errorf("Unexpected type: %T", item))
	}
}

// ResourceDataThumbnail resolves and returns the thumbnail for the resource at p.
func ResourceDataThumbnail(ctx context.Context, pt *path.Thumbnail, pd *path.ResourceData) (*image.Info, error) {
	obj, err := ResolveInternal(ctx, pd)
	if err != nil {
		return nil, err
	}

	t, ok := obj.(path.Thumbnailer)
	if !ok {
		return nil, fmt.Errorf("Type %T does not support thumbnailing", obj)
	}

	return t.Thumbnail(ctx, pt)
}
