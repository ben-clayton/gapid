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

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif  // __cplusplus

typedef struct arena_t arena;
typedef struct globals_t globals;
typedef struct string_t string;

#define GAPIL_APPLICATION_POOL 0

#define GAPIL_ERR_SUCCESS 0
#define GAPIL_ERR_ABORTED 1

#define GAPIL_MAP_ELEMENT_EMPTY 0
#define GAPIL_MAP_ELEMENT_FULL 1
#define GAPIL_MAP_ELEMENT_USED 2

#define GAPIL_MAP_GROW_MULTIPLIER 2
#define GAPIL_MIN_MAP_SIZE 16
#define GAPIL_MAP_MAX_CAPACITY 0.8f

// context contains information about the environment in which a function is
// executing.
typedef struct context_t {
  uint32_t id;        // the context identifier. Can be treated as user-data.
  uint32_t location;  // the API source location.
  uint64_t cmd_id;    // the current command identifier.
  globals* globals;   // a pointer to the global state.
  arena* arena;       // the memory arena used for allocations.
  void* arguments;    // the arguments to the currently executing command.
  // additional data used by compiler plugins goes here.
} context;

// slice is the data of a gapil slice type (elty foo[]).
typedef struct slice_t {
  uint64_t pool;   // the pool identifier. 0 represents the application pool.
  uint64_t root;   // original offset in bytes from pool base that this slice
                   // derives from.
  uint64_t base;   // offset in bytes from pool base of the first element.
  uint64_t size;   // size in bytes of the slice.
  uint64_t count;  // total number of elements in the slice.
} slice;

// string is the shared data of a gapil string type.
// A string is a pointer to this struct.
typedef struct string_t {
  uint32_t ref_count;  // number of owners of this string.
  arena* arena;        // arena that owns this string allocation.
  uint64_t length;  // size in bytes of this string (including null-terminator).
  uint8_t data[1];  // the null-terminated string bytes.
} string;

// map is the shared data of a gapil map type.
// A map is a pointer to this struct.
typedef struct map_t {
  uint32_t ref_count;  // number of owners of this map.
  arena* arena;  // arena that owns this map allocation and its elements buffer.
  uint64_t count;     // number of elements in the map.
  uint64_t capacity;  // size of the elements buffer.
  void* elements;     // pointer to the elements buffer.
} map;

// ref is the shared data of a gapil ref!T type.
// A ref is a pointer to this struct.
typedef struct ref_t {
  uint32_t ref_count;  // number of owners of this ref.
  arena* arena;        // arena that owns this ref allocation.
  /* T */              // referenced object immediately follows.
} ref;

// buffer is a structure used to hold a variable size byte array.
// buffer is used internally by the compiler to write out variable length data.
typedef struct buffer_t {
  uint8_t* data;      // buffer data.
  uint32_t capacity;  // total capacity of the buffer.
  uint32_t size;      // current size of the buffer.
} buffer;

typedef uint8_t GAPIL_BOOL;

#define GAPIL_FALSE 0
#define GAPIL_TRUE 1

#ifndef DECL_GAPIL_CB
#define DECL_GAPIL_CB(RETURN, NAME, ...) RETURN NAME(__VA_ARGS__)
#endif

////////////////////////////////////////////////////////////////////////////////
// Functions to be implemented by the user of the runtime                     //
////////////////////////////////////////////////////////////////////////////////

typedef enum gapil_data_access_t {
  GAPIL_READ = 0x1,
  GAPIL_WRITE = 0x2,
} gapil_data_access;

typedef struct gapil_runtime_callbacks_t {
  // applys the read observations tagged to the current command into the memory
  // model.
  void (*apply_reads)(context*);

  // applys the write observations tagged to the current command into the memory
  // model.
  void (*apply_writes)(context*);

  // Returns a pointer to the pool's data starting at pointer for size bytes.
  void* (*resolve_pool_data)(context*, uint64_t pool_id, uint64_t ptr,
                             gapil_data_access, uint64_t size);

  // stores the buffer at ptr of the given size into the database.
  // Writes the 20-byte database identifier of the stored data to id.
  void (*store_in_database)(context* ctx, void* ptr, uint64_t size,
                            uint8_t* id_out);

  // allocates a new pool with the given size in bytes and an initial reference
  // count of 1. The new pool's identifier is returned.
  uint64_t (*make_pool)(context*, uint64_t size);

  // increments the reference count of the given pool.
  void (*pool_reference)(context*, uint64_t pool_id);

  // decrements the reference count of the given pool, freeing it if the
  // reference count reaches 0.
  void (*pool_release)(context*, uint64_t pool_id);
} gapil_runtime_callbacks;

void gapil_set_runtime_callbacks(gapil_runtime_callbacks*);

////////////////////////////////////////////////////////////////////////////////
// Runtime API implemented by the compiler                                    //
////////////////////////////////////////////////////////////////////////////////

// creates an initializes a new context with the given arena.
context* gapil_create_context(arena* arena);

// destroys the context created by gapil_create_context.
void gapil_destroy_context(context*);

void gapil_string_reference(string*);
void gapil_string_release(string*);

void gapil_slice_reference(context*, slice);
void gapil_slice_release(context*, slice);

////////////////////////////////////////////////////////////////////////////////
// Runtime API implemented in runtime.cpp                                     //
////////////////////////////////////////////////////////////////////////////////

// allocates memory using the arena with the given size and alignment.
DECL_GAPIL_CB(void*, gapil_alloc, arena*, uint64_t size, uint64_t align);

// re-allocates memory previously allocated with the arena to a new size and
// alignment.
DECL_GAPIL_CB(void*, gapil_realloc, arena*, void* ptr, uint64_t size,
              uint64_t align);

// frees memory previously allocated with gapil_alloc or gapil_realloc.
DECL_GAPIL_CB(void, gapil_free, arena*, void* ptr);

// creates a buffer with the given alignment and capacity.
DECL_GAPIL_CB(void, gapil_create_buffer, arena*, uint64_t capacity,
              uint64_t alignment, buffer*);

// destroys a buffer previously created with gapil_create_buffer.
DECL_GAPIL_CB(void, gapil_destroy_buffer, arena*, buffer*);

// appends data to a buffer.
DECL_GAPIL_CB(void, gapil_append_buffer, arena*, buffer*, const void* data,
              uint64_t size, uint64_t alignment);

// returns a pointer to the underlying buffer data for the given slice,
// using gapil_data_resolver if it has been set.
DECL_GAPIL_CB(void*, gapil_slice_data, context*, slice*, gapil_data_access);

// copies N bytes of data from src to dst, where N is min(dst.size, src.size).
DECL_GAPIL_CB(void, gapil_copy_slice, context*, slice* dst, slice* src);

// allocates a new slice and underlying pool filled with the data of string.
DECL_GAPIL_CB(void, gapil_string_to_slice, context*, string* string,
              slice* out);

// allocates a new string with the given data and length.
// length excludes a null-pointer.
DECL_GAPIL_CB(string*, gapil_make_string, arena*, uint64_t length, void* data);

// outputs a slice spanning the bytes of the null-terminated string starting at
// ptr. The slice includes the null-terminator byte.
DECL_GAPIL_CB(void, gapil_cstring_to_slice, context*, uint64_t ptr, slice* out);

// frees a string allocated with gapil_make_string, gapil_string_concat or
// gapil_slice_to_string.
DECL_GAPIL_CB(void, gapil_free_string, string*);

// allocates a new string filled with the data of slice.
DECL_GAPIL_CB(string*, gapil_slice_to_string, context*, slice* slice);

// allocates a new string containing the concatenated data of the two strings.
DECL_GAPIL_CB(string*, gapil_string_concat, string*, string*);

// compares two strings lexicographically, using the same rules as strcmp.
DECL_GAPIL_CB(int32_t, gapil_string_compare, string*, string*);

// logs a message to the current logger.
// fmt is a printf-style message.
DECL_GAPIL_CB(void, gapil_logf, uint8_t severity, uint8_t* file, uint32_t line,
              uint8_t* fmt, ...);

// applys the read observations tagged to the current command into the memory
// model.
DECL_GAPIL_CB(void, gapil_apply_reads, context*);

// applys the write observations tagged to the current command into the memory
// model.
DECL_GAPIL_CB(void, gapil_apply_writes, context*);

// Returns a pointer to the pool's data starting at pointer for size bytes.
DECL_GAPIL_CB(void*, gapil_resolve_pool_data, context*, uint64_t pool_id,
              uint64_t ptr, gapil_data_access, uint64_t size);

// stores the buffer at ptr of the given size into the database.
// Writes the 20-byte database identifier of the stored data to id.
DECL_GAPIL_CB(void, gapil_store_in_database, context* ctx, void* ptr,
              uint64_t size, uint8_t* id_out);

// allocates a new pool with the given size in bytes and an initial reference
// count of 1. The new pool's identifier is returned.
DECL_GAPIL_CB(uint64_t, gapil_make_pool, context*, uint64_t size);

// increments the reference count of the given pool.
DECL_GAPIL_CB(void, gapil_pool_reference, context*, uint64_t pool_id);

// decrements the reference count of the given pool, freeing it if the reference
// count reaches 0.
DECL_GAPIL_CB(void, gapil_pool_release, context*, uint64_t pool_id);

#undef DECL_GAPIL_CB

#ifdef __cplusplus
}  // extern "C"
#endif  // __cplusplus

#endif  // __GAPIL_RUNTIME_H__