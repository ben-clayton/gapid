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

package gles

// #include "gapis/api/gles/ctypes.h"
import "C"

import (
	"unsafe"

	"github.com/google/gapid/gapil/executor"
)

func externsFromNative(ctx *C.context) *externs {
	env := executor.EnvFromNative(unsafe.Pointer(ctx))
	return &externs{
		ctx:   env.Context(),
		cmd:   env.Cmd(),
		cmdID: env.CmdID(),
		s:     env.State,
		b:     nil,
	}
}

//export gles_GetAndroidNativeBufferExtra
func gles_GetAndroidNativeBufferExtra(ctx *C.context, buffer uint64, out *unsafe.Pointer) {
	panic("gles_GetAndroidNativeBufferExtra not implemented")
}

//export gles_GetCompileShaderExtra
func gles_GetCompileShaderExtra(
	ctx *C.context,
	context *C.Context__R,
	shader *C.Shader__R,
	extra *C.BinaryExtra__R,
	out **C.CompileShaderExtra__R) {

	e := externsFromNative(ctx)
	*out = e.GetCompileShaderExtra(
		Contextʳ{context},
		Shaderʳ{shader},
		BinaryExtraʳ{extra},
	).c
}

//export gles_GetEGLDynamicContextState
func gles_GetEGLDynamicContextState(
	ctx *C.context,
	display EGLDisplay,
	surface EGLSurface,
	context EGLContext,
	out **C.DynamicContextState__R) {

	e := externsFromNative(ctx)
	*out = e.GetEGLDynamicContextState(display, surface, context).c
}

//export gles_GetEGLImageData
func gles_GetEGLImageData(ctx *C.context, img EGLImageKHR, width, height GLsizei) {
	e := externsFromNative(ctx)
	e.GetEGLImageData(img, width, height)
}

//export gles_GetEGLStaticContextState
func gles_GetEGLStaticContextState(
	ctx *C.context,
	display EGLDisplay,
	context EGLContext,
	out **C.StaticContextState__R) {

	e := externsFromNative(ctx)
	*out = e.GetEGLStaticContextState(display, context).c
}

//export gles_GetLinkProgramExtra
func gles_GetLinkProgramExtra(
	ctx *C.context,
	context *C.Context__R,
	program *C.Program__R,
	extra *C.BinaryExtra__R,
	out **C.LinkProgramExtra__R) {

	e := externsFromNative(ctx)
	*out = e.GetLinkProgramExtra(
		Contextʳ{context},
		Programʳ{program},
		BinaryExtraʳ{extra},
	).c
}

//export gles_GetValidateProgramExtra
func gles_GetValidateProgramExtra(
	ctx *C.context,
	context *C.Context__R,
	program *C.Program__R,
	out **C.ValidateProgramExtra__R) {

	e := externsFromNative(ctx)
	*out = e.GetValidateProgramExtra(
		Contextʳ{context},
		Programʳ{program},
	).c
}

//export gles_GetValidateProgramPipelineExtra
func gles_GetValidateProgramPipelineExtra(
	ctx *C.context,
	context *C.Context__R,
	pipeline *C.Pipeline__R,
	out **C.ValidateProgramPipelineExtra__R) {

	e := externsFromNative(ctx)
	*out = e.GetValidateProgramPipelineExtra(
		Contextʳ{context},
		Pipelineʳ{pipeline},
	).c
}

//export gles_IndexLimits
func gles_IndexLimits(ctx *C.context, s *C.slice, indexSize int32, out *C.u32Limits) {
	e := externsFromNative(ctx)
	*out = *e.IndexLimits(
		U8ˢ{s},
		indexSize,
	).c
}

//export gles_ReadGPUTextureData
func gles_ReadGPUTextureData(ctx *C.context, texture *C.Texture__R, level, layer GLint, out *C.slice) {
	e := externsFromNative(ctx)
	*out = *e.ReadGPUTextureData(
		Textureʳ{texture},
		level,
		layer,
	).c
}

//export gles_addTag
func gles_addTag(ctx *C.context, _ uint32, _ uint8) {
	// TODO: panic("gles_addTag not implemented")
}

//export gles_mapMemory
func gles_mapMemory(ctx *C.context, slice *C.slice) {
	e := externsFromNative(ctx)
	e.mapMemory(U8ˢ{slice})
}

//export gles_newMsg
func gles_newMsg(ctx *C.context, severity Severity, _ uint8, out *uint32) {
	// TODO: panic("gles_newMsg not implemented")
}

//export gles_onGlError
func gles_onGlError(ctx *C.context, err GLenum) {
	e := externsFromNative(ctx)
	e.onGlError(err)
}

//export gles_unmapMemory
func gles_unmapMemory(ctx *C.context, slice *C.slice) {
	e := externsFromNative(ctx)
	e.unmapMemory(U8ˢ{slice})
}
