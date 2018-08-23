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

package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/gapid/core/app"
	"github.com/google/gapid/core/app/crash"
	"github.com/google/gapid/core/app/status"
	"github.com/google/gapid/core/event/task"
	img "github.com/google/gapid/core/image"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/client"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
	"github.com/google/gapid/gapis/stringtable"
)

type benchmarkVerb struct {
	BenchmarkFlags
	startTime                          time.Time
	gapisStartTime                     time.Time
	gapisStringTableTime               time.Time
	serverInfoTime                     time.Time
	gotDevicesTime                     time.Time
	nDevices                           uint64
	foundTraceTargetTime               time.Time
	beforeStartTraceTime               time.Time
	traceInitializedTime               time.Time
	traceDoneTime                      time.Time
	traceSizeInBytes                   int64
	gapisTraceLoadTime                 time.Time
	gapisTraceLoadedTime               time.Time
	gapisGotEventsTime                 time.Time
	gapisGotResourcesTime              time.Time
	gapisGotContextsTime               time.Time
	gapisGotReplayDevicesTime          time.Time
	gapisReportTime                    time.Time
	gapisGotThumbnailsTime             time.Time
	gapisCommandTreeNodesResolved      time.Time
	gapisCommandTreeThumbnailsResolved time.Time
	interactionStartTime               time.Time
	interactionResolvedStateTree       time.Time
	interactionFramebufferTime         time.Time
	interactionMeshTime                time.Time
	interactionResourcesTime           time.Time
	interactionMemoryTime              time.Time
	interactionDoneTime                time.Time
	traceFrames                        int
}

var BenchmarkName = "benchmark.gfxtrace"

func init() {
	verb := &benchmarkVerb{}

	app.AddVerb(&app.Verb{
		Name:      "benchmark",
		ShortHelp: "Runs a set of benchmarking tests on an application",
		Action:    verb,
	})
}

func (verb *benchmarkVerb) Run(ctx context.Context, flags flag.FlagSet) error {
	oldCtx := ctx
	ctx = status.Start(ctx, "Initializing GAPIS")

	if verb.For.Seconds() == float64(0) {
		verb.For = time.Duration(time.Minute)
	}

	verb.startTime = time.Now()

	client, err := getGapis(ctx, GapisFlags{}, GapirFlags{})
	verb.gapisStartTime = time.Now()
	if verb.DumpTrace != "" {
		profile := bytes.Buffer{}
		stopProfile := status.RegisterTracer(&profile)
		trace, err := os.Create(verb.DumpTrace)
		if err != nil {
			panic(err)
		}
		defer func() {
			// Skip the leading [
			stopProfile()
			trace.Write(profile.Bytes()[1:])
			trace.Close()
		}()
		stop, err := client.Profile(ctx, nil, trace, 1)
		if err != nil {
			panic(err)
		}
		defer stop()
	}

	stringTables, err := client.GetAvailableStringTables(ctx)
	if err != nil {
		return log.Err(ctx, err, "Failed get list of string tables")
	}

	var stringTable *stringtable.StringTable
	if len(stringTables) > 0 {
		// TODO: Let the user pick the string table.
		stringTable, err = client.GetStringTable(ctx, stringTables[0])
		if err != nil {
			return log.Err(ctx, err, "Failed get string table")
		}
	}
	_ = stringTable
	verb.gapisStringTableTime = time.Now()

	if err != nil {
		return log.Err(ctx, err, "Failed to connect to the GAPIS server")
	}
	defer client.Close()
	status.Finish(ctx)

	if flags.NArg() > 0 {
		traceURI := flags.Arg(0)
		verb.doTrace(ctx, client, traceURI)
		verb.traceDoneTime = time.Now()
	}

	s, err := os.Stat(BenchmarkName)
	if err != nil {
		return err
	}

	verb.traceSizeInBytes = s.Size()
	status.Event(ctx, status.GlobalScope, "Trace Size %+v", verb.traceSizeInBytes)

	ctx = status.Start(oldCtx, "Initializing Capture")
	verb.gapisTraceLoadTime = time.Now()
	c, err := client.LoadCapture(ctx, BenchmarkName)
	if err != nil {
		return err
	}
	verb.gapisTraceLoadedTime = time.Now()

	devices, err := client.GetDevicesForReplay(ctx, c)
	if err != nil {
		panic(err)
	}
	if len(devices) == 0 {
		panic("No devices")
	}

	resolveConfig := &path.ResolveConfig{
		ReplayDevice: devices[0],
	}
	device := devices[0]

	verb.gapisGotReplayDevicesTime = time.Now()

	wg := sync.WaitGroup{}
	gotContext := sync.WaitGroup{}

	var resources *service.Resources
	wg.Add(1)
	go func() {
		boxedResources, err := client.Get(ctx, c.Resources().Path(), resolveConfig)
		if err != nil {
			panic(err)
		}
		resources = boxedResources.(*service.Resources)
		verb.gapisGotResourcesTime = time.Now()

		wg.Done()
	}()

	var context *service.Context
	var ctxId *path.ID

	wg.Add(1)
	gotContext.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Resolving Contexts")
		defer status.Finish(ctx)
		contextsInterface, err := client.Get(ctx, c.Contexts().Path(), resolveConfig)
		if err != nil {
			panic(err)
		}
		contexts := contextsInterface.(*service.Contexts)
		fmt.Printf("%+v\n", contexts.GetList()[0])
		ctxId = contexts.GetList()[0].ID
		contextInterface, err := client.Get(ctx, contexts.GetList()[0].Path(), resolveConfig)
		context = contextInterface.(*service.Context)

		verb.gapisGotContextsTime = time.Now()
		gotContext.Done()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Getting Report")
		defer status.Finish(ctx)
		gotContext.Wait()
		filter := &path.CommandFilter{}
		filter.Context = ctxId

		_, err := client.Get(ctx, c.Commands().Path(), resolveConfig)
		if err != nil {
			panic(err)
		}

		_, err = client.Get(ctx, c.Report(device, filter, false).Path(), resolveConfig)
		verb.gapisReportTime = time.Now()
		wg.Done()
	}()

	var commandToClick *path.Command

	wg.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Getting Thumbnails")
		defer status.Finish(ctx)
		events, err := getEvents(ctx, client, &path.Events{
			Capture:                 c,
			AllCommands:             false,
			FirstInFrame:            false,
			LastInFrame:             true,
			FramebufferObservations: false,
		})
		if err != nil {
			panic(err)
		}
		verb.gapisGotEventsTime = time.Now()
		verb.traceFrames = len(events)

		gotThumbnails := sync.WaitGroup{}
		//Get thumbnails
		settings := &service.RenderSettings{MaxWidth: uint32(256), MaxHeight: uint32(256)}
		numThumbnails := 10
		if len(events) < 10 {
			numThumbnails = len(events)
		}
		commandToClick = events[len(events)-1].Command
		for i := len(events) - numThumbnails; i < len(events); i++ {
			gotThumbnails.Add(1)
			hints := &service.UsageHints{Preview: true}
			go func(i int) {
				iip, err := client.GetFramebufferAttachment(ctx,
					&service.ReplaySettings{
						Device: device,
						DisableReplayOptimization: false,
						DisplayToSurface:          false,
					},
					events[i].Command, api.FramebufferAttachment_Color0, settings, hints)

				iio, err := client.Get(ctx, iip.Path(), resolveConfig)
				if err != nil {
					panic(log.Errf(ctx, err, "Get frame image.Info failed"))
				}
				ii := iio.(*img.Info)
				dataO, err := client.Get(ctx, path.NewBlob(ii.Bytes.ID()).Path(), resolveConfig)
				if err != nil {
					panic(log.Errf(ctx, err, "Get frame image data failed"))
				}
				_, _, _ = int(ii.Width), int(ii.Height), dataO.([]byte)
				gotThumbnails.Done()
			}(i)
		}
		gotThumbnails.Wait()
		verb.gapisGotThumbnailsTime = time.Now()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Resolving Command Tree")

		gotContext.Wait()
		filter := &path.CommandFilter{}
		filter.Context = ctxId

		treePath := c.CommandTree(filter)
		treePath.GroupByApi = true
		treePath.GroupByContext = true
		treePath.GroupByDrawCall = true
		treePath.GroupByFrame = true
		treePath.GroupByUserMarkers = true
		treePath.IncludeNoContextGroups = true
		treePath.AllowIncompleteFrame = true
		treePath.MaxChildren = int32(2000)

		boxedTree, err := client.Get(ctx, treePath.Path(), resolveConfig)
		if err != nil {
			panic(log.Err(ctx, err, "Failed to load the command tree"))
		}
		tree := boxedTree.(*service.CommandTree)

		boxedNode, err := client.Get(ctx, tree.Root.Path(), resolveConfig)
		if err != nil {
			panic(log.Errf(ctx, err, "Failed to load the node at: %v", tree.Root.Path()))
		}

		n := boxedNode.(*service.CommandTreeNode)
		numChildren := 30
		if n.NumChildren < 30 {
			numChildren = int(n.NumChildren)
		}
		gotThumbnails := sync.WaitGroup{}
		gotNodes := sync.WaitGroup{}
		settings := &service.RenderSettings{MaxWidth: uint32(64), MaxHeight: uint32(64)}
		hints := &service.UsageHints{Background: true}
		tnCtx := status.Start(oldCtx, "Resolving Command Thumbnails")
		for i := 0; i < numChildren; i++ {
			gotThumbnails.Add(1)
			gotNodes.Add(1)
			go func(i int) {
				defer gotThumbnails.Done()
				boxedChild, err := client.Get(ctx, tree.Root.Child(uint64(i)).Path(), resolveConfig)
				if err != nil {
					panic(err)
				}
				child := boxedChild.(*service.CommandTreeNode)
				gotNodes.Done()
				iip, err := client.GetFramebufferAttachment(tnCtx,
					&service.ReplaySettings{
						Device: device,
						DisableReplayOptimization: false,
						DisplayToSurface:          false,
					},
					child.Representation, api.FramebufferAttachment_Color0, settings, hints)

				iio, err := client.Get(tnCtx, iip.Path(), resolveConfig)
				if err != nil {
					return
				}
				ii := iio.(*img.Info)
				dataO, err := client.Get(tnCtx, path.NewBlob(ii.Bytes.ID()).Path(), resolveConfig)
				if err != nil {
					panic(log.Errf(tnCtx, err, "Get frame image data failed"))
				}
				_, _, _ = int(ii.Width), int(ii.Height), dataO.([]byte)
			}(i)
		}

		gotNodes.Wait()
		status.Finish(ctx)
		verb.gapisCommandTreeNodesResolved = time.Now()

		gotThumbnails.Wait()
		status.Finish(tnCtx)
		verb.gapisCommandTreeThumbnailsResolved = time.Now()
		wg.Done()
	}()
	// Done initializing capture
	wg.Wait()
	// At this point we are Interactive. All pre-loading is done:
	// Next we have to actually handle an interaction
	status.Finish(ctx)

	status.Event(ctx, status.GlobalScope, "Load done, interaction starting %+v", verb.traceSizeInBytes)

	ctx = status.Start(oldCtx, "Interacting with frame")
	// One interaction done
	verb.interactionStartTime = time.Now()
	interactionWG := sync.WaitGroup{}
	interactionWG.Add(1)
	// Get state tree
	go func() {
		ctx = status.Start(oldCtx, "Resolving State Tree")
		defer status.Finish(ctx)
		defer interactionWG.Done()
		//commandToClick
		boxedTree, err := client.Get(ctx, commandToClick.StateAfter().Tree().Path(), resolveConfig)
		if err != nil {
			panic(log.Err(ctx, err, "Failed to load the state tree"))
		}
		tree := boxedTree.(*service.StateTree)

		boxedRoot, err := client.Get(ctx, tree.Root.Path(), resolveConfig)
		if err != nil {
			panic(log.Err(ctx, err, "Failed to load the state tree"))
		}
		root := boxedRoot.(*service.StateTreeNode)

		gotNodes := sync.WaitGroup{}
		numChildren := 30
		if root.NumChildren < 30 {
			numChildren = int(root.NumChildren)
		}
		for i := 0; i < numChildren; i++ {
			gotNodes.Add(1)
			go func(i int) {
				defer gotNodes.Done()
				boxedChild, err := client.Get(ctx, tree.Root.Index(uint64(i)).Path(), resolveConfig)
				if err != nil {
					panic(err)
				}
				child := boxedChild.(*service.StateTreeNode)

				if child.Preview != nil {
					if child.Constants != nil {
						_, _ = getConstantSet(ctx, client, child.Constants)
					}
				}
			}(i)
		}
		gotNodes.Wait()
		verb.interactionResolvedStateTree = time.Now()
	}()

	// Get the framebuffer
	interactionWG.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Getting Framebuffer")
		defer status.Finish(ctx)
		defer interactionWG.Done()
		hints := &service.UsageHints{Primary: true}
		settings := &service.RenderSettings{MaxWidth: uint32(0xFFFFFFFF), MaxHeight: uint32(0xFFFFFFFF)}
		iip, err := client.GetFramebufferAttachment(ctx,
			&service.ReplaySettings{
				Device: device,
				DisableReplayOptimization: false,
				DisplayToSurface:          false,
			},
			commandToClick, api.FramebufferAttachment_Color0, settings, hints)

		iio, err := client.Get(ctx, iip.Path(), resolveConfig)
		if err != nil {
			return
		}
		ii := iio.(*img.Info)
		dataO, err := client.Get(ctx, path.NewBlob(ii.Bytes.ID()).Path(), resolveConfig)
		if err != nil {
			panic(log.Errf(ctx, err, "Get frame image data failed"))
		}
		_, _, _ = int(ii.Width), int(ii.Height), dataO.([]byte)
		verb.interactionFramebufferTime = time.Now()
	}()

	// Get the mesh
	interactionWG.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Getting Mesh")
		defer status.Finish(ctx)
		defer interactionWG.Done()
		meshOptions := path.NewMeshOptions(false)
		_, _ = client.Get(ctx, commandToClick.Mesh(meshOptions).Path(), resolveConfig)
		verb.interactionMeshTime = time.Now()
	}()

	// GetMemory
	interactionWG.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Getting Memory")
		defer status.Finish(ctx)
		defer interactionWG.Done()
		observationsPath := &path.Memory{
			Address:         0,
			Size:            uint64(0xFFFFFFFFFFFFFFFF),
			Pool:            0,
			After:           commandToClick,
			ExcludeData:     true,
			ExcludeObserved: true,
		}
		allMemory, err := client.Get(ctx, observationsPath.Path(), resolveConfig)
		if err != nil {
			panic(err)
		}
		memory := allMemory.(*service.Memory)
		gotMemory := sync.WaitGroup{}
		for _, x := range memory.Reads {
			gotMemory.Add(1)
			go func(addr, size uint64) {
				defer gotMemory.Done()
				client.Get(ctx, commandToClick.MemoryAfter(0, addr, size).Path(), resolveConfig)
			}(x.Base, x.Size)
		}
		for _, x := range memory.Writes {
			gotMemory.Add(1)
			go func(addr, size uint64) {
				defer gotMemory.Done()
				client.Get(ctx, commandToClick.MemoryAfter(0, addr, size).Path(), resolveConfig)
			}(x.Base, x.Size)
		}
		gotMemory.Wait()
	}()

	// Get Resource Data (For each texture, and shader)
	interactionWG.Add(1)
	go func() {
		ctx = status.Start(oldCtx, "Getting Resources")
		defer status.Finish(ctx)
		defer interactionWG.Done()
		gotResources := sync.WaitGroup{}
		for _, types := range resources.GetTypes() {
			for ii, v := range types.GetResources() {
				if (types.Type == api.ResourceType_TextureResource ||
					types.Type == api.ResourceType_ShaderResource ||
					types.Type == api.ResourceType_ProgramResource) &&
					ii < 30 {
					gotResources.Add(1)
					go func(id *path.ID) {
						defer gotResources.Done()
						resourcePath := commandToClick.ResourceAfter(id)
						_, _ = client.Get(ctx, resourcePath.Path(), resolveConfig)
					}(v.ID)
				}
			}
		}
		gotResources.Wait()
		verb.interactionResourcesTime = time.Now()
	}()

	interactionWG.Wait()
	verb.interactionDoneTime = time.Now()
	status.Finish(ctx)
	ctx = oldCtx
	fmt.Printf("Gapis creation time %+v\n", (verb.gapisStartTime.Sub(verb.startTime)))
	fmt.Printf("Gapis get string table time %+v\n", verb.gapisStringTableTime.Sub(verb.gapisStartTime))
	fmt.Printf("Get Server Info Time %+v\n", (verb.serverInfoTime.Sub(verb.gapisStringTableTime)))
	fmt.Printf("Start until Devices enumerated %+v\n", (verb.serverInfoTime.Sub(verb.startTime)))
	fmt.Printf("Finding the correct target %+v\n", (verb.foundTraceTargetTime.Sub(verb.gotDevicesTime)))
	fmt.Printf("Setting up trace %+v\n", (verb.traceInitializedTime.Sub(verb.beforeStartTraceTime)))
	fmt.Printf("Trace setup time: : %+v\n", verb.traceDoneTime.Sub(verb.traceInitializedTime)-verb.For)
	fmt.Printf("Total trace time: %+v\n", (verb.traceDoneTime.Sub(verb.traceInitializedTime)))
	fmt.Printf("Trace Size %+vMB\n", (verb.traceSizeInBytes / (1024 * 1024)))
	fmt.Printf("Total frames captured %+v\n", verb.traceFrames)
	fmt.Printf("Frame Time %+v\n", (verb.For.Seconds() / float64(verb.traceFrames)))

	fmt.Printf("Server Trace Load Time %+v\n", verb.gapisTraceLoadedTime.Sub(verb.gapisTraceLoadTime))
	fmt.Printf("Resolved Replay Device Time %+v\n", verb.gapisGotReplayDevicesTime.Sub(verb.gapisTraceLoadedTime))
	fmt.Printf("Resolved Resources Time %+v\n", verb.gapisGotResourcesTime.Sub(verb.gapisGotReplayDevicesTime))
	fmt.Printf("Got Contexts Time %+v\n", verb.gapisGotContextsTime.Sub(verb.gapisGotReplayDevicesTime))
	fmt.Printf("Report completed time %+v\n", verb.gapisReportTime.Sub(verb.gapisGotReplayDevicesTime))
	fmt.Printf("Thumbnails completed time %+v\n", verb.gapisGotThumbnailsTime.Sub(verb.gapisGotReplayDevicesTime))
	fmt.Printf("Command Tree Nodes time %+v\n", verb.gapisCommandTreeNodesResolved.Sub(verb.gapisGotReplayDevicesTime))
	fmt.Printf("Command Tree Thumbnails time %+v\n", verb.gapisCommandTreeThumbnailsResolved.Sub(verb.gapisGotReplayDevicesTime))

	fmt.Printf("Interaction Command Index: %+v\n", commandToClick.Indices[0])
	fmt.Printf("Interactions: State Tree: %+v\n", verb.interactionResolvedStateTree.Sub(verb.interactionStartTime))
	fmt.Printf("Interactions: Framebuffer: %+v\n", verb.interactionFramebufferTime.Sub(verb.interactionStartTime))
	fmt.Printf("Interactions: Mesh: %+v\n", verb.interactionMeshTime.Sub(verb.interactionStartTime))
	fmt.Printf("Interactions: Resources: %+v\n", verb.interactionResourcesTime.Sub(verb.interactionStartTime))
	fmt.Printf("Interactions Done: %+v\n", verb.interactionDoneTime.Sub(verb.interactionStartTime))

	return nil
}

// This intentionally duplicates a lot of the gapit trace logic
// so that we can independently chnage how what we do to benchmark
// everything.
func (verb *benchmarkVerb) doTrace(ctx context.Context, client client.Client, traceURI string) error {
	ctx = status.Start(ctx, "Record Trace for %+v", verb.For)
	defer status.Finish(ctx)

	// Find the actual trace URI from all of the devices
	_, err := client.GetServerInfo(ctx)
	if err != nil {
		return err
	}
	verb.serverInfoTime = time.Now()

	devices, err := client.GetDevices(ctx)
	if err != nil {
		return err
	}
	verb.gotDevicesTime = time.Now()
	verb.nDevices = uint64(len(devices))

	devices, err = filterDevices(ctx, &verb.DeviceFlags, client)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("Could not find matching device")
	}

	type info struct {
		uri        string
		device     *path.Device
		deviceName string
		name       string
	}
	var found []info

	for _, dev := range devices {
		targets, err := client.FindTraceTargets(ctx, &service.FindTraceTargetsRequest{
			Device: dev,
			Uri:    traceURI,
		})
		if err != nil {
			continue
		}

		dd, err := client.Get(ctx, dev.Path(), nil)
		if err != nil {
			return err
		}
		d := dd.(*device.Instance)

		for _, target := range targets {
			name := target.Name
			switch {
			case target.FriendlyApplication != "":
				name = target.FriendlyApplication
			case target.FriendlyExecutable != "":
				name = target.FriendlyExecutable
			}

			found = append(found, info{
				uri:        target.Uri,
				deviceName: d.Name,
				device:     dev,
				name:       name,
			})
		}
	}

	if len(found) == 0 {
		return fmt.Errorf("Could not find %+v to trace on any device", traceURI)
	}

	if len(found) > 1 {
		sb := strings.Builder{}
		fmt.Fprintf(&sb, "Found %v candidates: \n", traceURI)
		for i, f := range found {
			if i == 0 || found[i-1].deviceName != f.deviceName {
				fmt.Fprintf(&sb, "  %v:\n", f.deviceName)
			}
			fmt.Fprintf(&sb, "    %v\n", f.uri)
		}
		return log.Errf(ctx, nil, "%v", sb.String())
	}

	fmt.Printf("Tracing %+v", found[0].uri)
	out := BenchmarkName
	uri := found[0].uri
	traceDevice := found[0].device

	verb.foundTraceTargetTime = time.Now()

	options := &service.TraceOptions{
		Device: traceDevice,
		Apis:   []string{},
		AdditionalCommandLineArgs: verb.AdditionalArgs,
		Cwd:                   verb.WorkingDir,
		Environment:           verb.Env,
		Duration:              float32(verb.For.Seconds()),
		ObserveFrameFrequency: 0,
		ObserveDrawFrequency:  0,
		StartFrame:            0,
		FramesToCapture:       0,
		DisablePcs:            true,
		RecordErrorState:      false,
		DeferStart:            false,
		NoBuffer:              false,
		HideUnknownExtensions: true,
		ClearCache:            false,
		ServerLocalSavePath:   out,
	}
	options.App = &service.TraceOptions_Uri{
		uri,
	}
	switch verb.API {
	case "vulkan":
		options.Apis = []string{"Vulkan"}
	case "gles":
		// TODO: Separate these two out once we can trace Vulkan with OpenGL ES.
		options.Apis = []string{"OpenGLES", "GVR"}
	case "":
		options.Apis = []string{"Vulkan", "OpenGLES", "GVR"}
	default:
		return fmt.Errorf("Unknown API %s", verb.API)
	}
	verb.beforeStartTraceTime = time.Now()
	handler, err := client.Trace(ctx)
	if err != nil {
		return err
	}
	defer handler.Dispose()

	defer app.AddInterruptHandler(func() {
		handler.Dispose()
	})()

	status, err := handler.Initialize(options)
	if err != nil {
		return err
	}
	verb.traceInitializedTime = time.Now()

	handlerInstalled := false

	return task.Retry(ctx, 0, time.Second*3, func(ctx context.Context) (retry bool, err error) {
		status, err = handler.Event(service.TraceEvent_Status)
		if err == io.EOF {
			return true, nil
		}
		if err != nil {
			log.I(ctx, "Error %+v", err)
			return true, err
		}
		if status == nil {
			return true, nil
		}

		if status.BytesCaptured > 0 {
			if !handlerInstalled {
				crash.Go(func() {
					reader := bufio.NewReader(os.Stdin)
					if options.DeferStart {
						println("Press enter to start capturing...")
						_, _ = reader.ReadString('\n')
						_, _ = handler.Event(service.TraceEvent_Begin)
					}
					println("Press enter to stop capturing...")
					_, _ = reader.ReadString('\n')
					handler.Event(service.TraceEvent_Stop)
				})
				handlerInstalled = true
			}
			log.I(ctx, "Capturing %+v", status.BytesCaptured)
		}
		if status.Status == service.TraceStatus_Done {
			return true, nil
		}
		return false, nil
	})
}
