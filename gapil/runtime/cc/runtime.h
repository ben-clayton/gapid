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

#ifndef __GAPIL_RUNTIME_H__
#define __GAPIL_RUNTIME_H__

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif // __cplusplus

typedef struct arena_t   arena;
typedef struct pool_t    pool;
typedef struct globals_t globals;
typedef struct string_t  string;

#define GAPIL_ERR_SUCCESS 0
#define GAPIL_ERR_ABORTED 1

#define GAPIL_MAP_ELEMENT_EMPTY 0
#define GAPIL_MAP_ELEMENT_FULL  1
#define GAPIL_MAP_ELEMENT_USED  2

#define GAPIL_MAP_GROW_MULTIPLIER 2
#define GAPIL_MIN_MAP_SIZE        16
#define GAPIL_MAP_MAX_CAPACITY    0.8f

typedef struct context_t {
	uint32_t    id;
	uint32_t    location;
	uint32_t    next_pool_id;
	globals*    globals;
	arena*      arena;
} context;

typedef struct pool_t {
	uint32_t ref_count;
	uint32_t id;     // Unique identifier of this pool.
	uint64_t size;   // Total size of the pool in bytes.
	arena*   arena;  // arena that owns the allocation of this pool and its buffer.
	void*    buffer; // nullptr for application pool
} pool;

typedef struct slice_t {
	pool*    pool; // The underlying pool. nullptr represents the application pool.
	void*    root; // Original pointer this slice derives from.
	void*    base; // Address of first element.
	uint64_t size; // Size in bytes of the slice.
} slice;

typedef struct string_t {
	uint32_t ref_count;
	arena*   arena; // arena that owns this string allocation.
	uint64_t length;
	uint8_t  data[1];
} string;

typedef struct map_t {
	uint32_t ref_count;
	arena*   arena; // arena that owns this map allocation and its elements buffer.
	uint64_t count;
	uint64_t capacity;
	void*    elements;
} map;

////////////////////////////////////////////////////////////////////////////////
// Functions to be implemented by the user of the runtime                     //
////////////////////////////////////////////////////////////////////////////////
void* gapil_remap_pointer(context* ctx, uint64_t pointer, uint64_t length);
void  gapil_get_code_location(context* ctx, char** file, uint32_t* line);

////////////////////////////////////////////////////////////////////////////////
// Runtime API                                                                //
////////////////////////////////////////////////////////////////////////////////
void gapil_init_context(context* ctx);
void gapil_term_context(context* ctx);

#ifndef DECL_GAPIL_CALLBACK
#define DECL_GAPIL_CALLBACK(RETURN, NAME, ...) RETURN NAME(__VA_ARGS__)
#endif

DECL_GAPIL_CALLBACK(void*,   gapil_alloc,          arena*, uint64_t size, uint64_t align);
DECL_GAPIL_CALLBACK(void*,   gapil_realloc,        arena*, void* ptr, uint64_t size, uint64_t align);
DECL_GAPIL_CALLBACK(void,    gapil_free,           arena*, void* ptr);
DECL_GAPIL_CALLBACK(void,    gapil_free_pool,      pool*);
DECL_GAPIL_CALLBACK(string*, gapil_make_string,    arena*, uint64_t length, void* data);
DECL_GAPIL_CALLBACK(void,    gapil_free_string,    string*);
DECL_GAPIL_CALLBACK(string*, gapil_string_concat,  string*, string*);
DECL_GAPIL_CALLBACK(int32_t, gapil_string_compare, string*, string*);

DECL_GAPIL_CALLBACK(void,    gapil_apply_reads,       context* ctx);
DECL_GAPIL_CALLBACK(void,    gapil_apply_writes,      context* ctx);
DECL_GAPIL_CALLBACK(pool*,   gapil_make_pool,         context* ctx, uint64_t size);
DECL_GAPIL_CALLBACK(void,    gapil_make_slice,        context* ctx, uint64_t size, slice* out);
DECL_GAPIL_CALLBACK(void,    gapil_copy_slice,        context* ctx, slice* dst, slice* src);
DECL_GAPIL_CALLBACK(void,    gapil_pointer_to_slice,  context* ctx, uint64_t ptr, uint64_t offset, uint64_t size, slice* out);
DECL_GAPIL_CALLBACK(string*, gapil_pointer_to_string, context* ctx, uint64_t ptr);
DECL_GAPIL_CALLBACK(string*, gapil_slice_to_string,   context* ctx, slice* slice);
DECL_GAPIL_CALLBACK(void,    gapil_string_to_slice,   context* ctx, string* string, slice* out);
DECL_GAPIL_CALLBACK(void,    gapil_call_extern,       context* ctx, string* name, void* args, void* res);
DECL_GAPIL_CALLBACK(void,    gapil_logf,              context* ctx, uint8_t severity, uint8_t* fmt, ...);

#undef DECL_GAPIL_CALLBACK

#ifdef __cplusplus
} // extern "C"
#endif // __cplusplus

#endif  // __GAPIL_RUNTIME_H__