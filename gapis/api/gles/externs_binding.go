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
	env := executor.GetEnv(unsafe.Pointer(ctx))
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
	c := Contextʳ{context}
	s := Shaderʳ{shader}
	b := BinaryExtraʳ{extra}
	*out = e.GetCompileShaderExtra(c, s, b).c
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
func gles_GetEGLImageData(ctx *C.context, img uint64, width int32, height int32) {
	panic("gles_GetEGLImageData not implemented")
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
func gles_GetLinkProgramExtra(ctx, c, p, binary unsafe.Pointer, out *unsafe.Pointer) {
	panic("gles_GetLinkProgramExtra not implemented")
}

//export gles_GetValidateProgramExtra
func gles_GetValidateProgramExtra(ctx, c, p unsafe.Pointer, out *unsafe.Pointer) {
	panic("gles_GetValidateProgramExtra not implemented")
}

//export gles_GetValidateProgramPipelineExtra
func gles_GetValidateProgramPipelineExtra(ctx, c, p, out *unsafe.Pointer) {
	panic("gles_GetValidateProgramPipelineExtra not implemented")
}

//export gles_IndexLimits
func gles_IndexLimits(ctx, s unsafe.Pointer, sizeofIndex int32, out *unsafe.Pointer) {
	panic("gles_IndexLimits not implemented")
}

//export gles_ReadGPUTextureData
func gles_ReadGPUTextureData(ctx, t unsafe.Pointer, level int32, layer int32, out *unsafe.Pointer) {
	panic("gles_ReadGPUTextureData not implemented")
}

//export gles_addTag
func gles_addTag(ctx *C.context, _ uint32, _ uint8) {
	panic("gles_addTag not implemented")
}

//export gles_mapMemory
func gles_mapMemory(ctx, s unsafe.Pointer) {
	panic("gles_mapMemory not implemented")
}

//export gles_newMsg
func gles_newMsg(ctx *C.context, _ uint32, _ uint8, out *uint32) {
	panic("gles_newMsg not implemented")
}

//export gles_onGlError
func gles_onGlError(ctx *C.context, err GLenum) {
	panic("gles_onGlError not implemented")
}

//export gles_unmapMemory
func gles_unmapMemory(ctx, s unsafe.Pointer) {
	panic("gles_unmapMemory not implemented")
}
