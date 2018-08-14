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

#include "runtime.h"

#include "core/cc/assert.h"
#include "core/cc/log.h"
#include "core/memory/arena/cc/arena.h"

#include <stdarg.h>
#include <stddef.h>
#include <stdlib.h>

#if TARGET_OS == GAPID_OS_ANDROID
// for snprintf
#include <cstdio>
#endif

#define __STDC_FORMAT_MACROS
#include <inttypes.h>

#include <cstring>

#if 0
#define DEBUG_PRINT(...) GAPID_WARNING(__VA_ARGS__)
#else
#define DEBUG_PRINT(...)
#endif

#define SLICE_FMT                                             \
  "[pool: %" PRIu64 ", root: 0x%" PRIx64 ", base: 0x%" PRIx64 \
  ", size: 0x%" PRIx64 ", count: 0x%" PRIx64 "]"
#define SLICE_ARGS(sli) sli->pool, sli->root, sli->base, sli->size, sli->count

using core::Arena;

namespace {

gapil_runtime_callbacks runtime_callbacks = {0};

}  // anonymous namespace

extern "C" {

void gapil_set_runtime_callbacks(gapil_runtime_callbacks* cbs) {
  runtime_callbacks = *cbs;
}

void gapil_logf(uint8_t severity, uint8_t* file, uint32_t line, uint8_t* fmt,
                ...) {
  if (GAPID_SHOULD_LOG(severity)) {
    va_list args;
    va_start(args, fmt);
    auto f =
        (file != nullptr) ? reinterpret_cast<const char*>(file) : "<unknown>";
#if TARGET_OS == GAPID_OS_ANDROID
    char buf[2048];
    snprintf(buf, sizeof(buf), "[%s:%" PRIu32 "] %s", f, line, fmt);
    __android_log_vprint(severity, "GAPID", buf, args);
#else
    ::core::Logger::instance().vlogf(severity, f, line,
                                     reinterpret_cast<const char*>(fmt), args);
#endif  // TARGET_OS
    va_end(args);
  }
}

void* gapil_alloc(arena_t* a, uint64_t size, uint64_t align) {
  Arena* arena = reinterpret_cast<Arena*>(a);
  void* ptr = arena->allocate(size, align);
  memset(ptr, 0, size);
  DEBUG_PRINT("gapil_alloc(size: 0x%" PRIx64 ", align: 0x%" PRIx64 ") -> %p",
              size, align, ptr);
  return ptr;
}

void* gapil_realloc(arena_t* a, void* ptr, uint64_t size, uint64_t align) {
  Arena* arena = reinterpret_cast<Arena*>(a);
  void* retptr = arena->reallocate(ptr, size, align);
  DEBUG_PRINT("gapil_realloc(ptr: %p, 0x%" PRIx64 ", align: 0x%" PRIx64
              ") -> %p",
              ptr, size, align, retptr);
  return retptr;
}

void gapil_free(arena_t* a, void* ptr) {
  DEBUG_PRINT("gapil_free(ptr: %p)", ptr);

  Arena* arena = reinterpret_cast<Arena*>(a);
  arena->free(ptr);
}

void gapil_create_buffer(arena* a, uint64_t capacity, uint64_t alignment,
                         buffer* buf) {
  DEBUG_PRINT("gapil_create_buffer(capacity: %" PRId64 ", alignment: %" PRId64
              ")",
              capacity, alignment);
  Arena* arena = reinterpret_cast<Arena*>(a);
  buf->data = (uint8_t*)arena->allocate(capacity, alignment);
  buf->size = 0;
  buf->capacity = capacity;
}

void gapil_destroy_buffer(arena* a, buffer* buf) {
  DEBUG_PRINT("gapil_destroy_buffer()");
  Arena* arena = reinterpret_cast<Arena*>(a);
  arena->free(buf->data);
  buf->capacity = 0;
  buf->size = 0;
}

void gapil_append_buffer(arena* a, buffer* buf, const void* data, uint64_t size,
                         uint64_t alignment) {
  DEBUG_PRINT("gapil_append_buffer(data: %p, size: %" PRId64
              ", alignment: %" PRId64 ")",
              data, size, alignment);
  if (buf->size + size > buf->capacity) {
    Arena* arena = reinterpret_cast<Arena*>(a);
    buf->capacity *= 2;
    buf->data =
        (uint8_t*)arena->reallocate(buf->data, buf->capacity, alignment);
  }
  memcpy(buf->data + buf->size, data, size);
  buf->size = buf->size + size;
}

void* gapil_slice_data(context* ctx, slice* sli, gapil_data_access access) {
  auto ptr =
      gapil_resolve_pool_data(ctx, sli->pool, sli->base, access, sli->size);
  DEBUG_PRINT("gapil_slice_data(" SLICE_FMT ", %d) -> %p", SLICE_ARGS(sli),
              access, ptr);
  return ptr;
}

string* gapil_make_string(arena* a, uint64_t length, void* data) {
  Arena* arena = reinterpret_cast<Arena*>(a);

  auto str = reinterpret_cast<string_t*>(
      arena->allocate(sizeof(string_t) + length + 1, 1));
  str->arena = a;
  str->ref_count = 1;
  str->length = length;

  if (data != nullptr) {
    memcpy(str->data, data, length);
    str->data[length] = 0;
  } else {
    memset(str->data, 0, length + 1);
  }

  DEBUG_PRINT("gapil_make_string(arena: %p, length: %" PRIu64
              ", data: '%s') -> %p",
              a, length, data, str);

  return str;
}

void gapil_free_string(string* str) {
  DEBUG_PRINT("gapil_free_string(str: %p, ref_count: %" PRIu32 ", len: %" PRIu64
              ", str: '%s' (%p))",
              str, str->ref_count, str->length, str->data, str->data);

  Arena* arena = reinterpret_cast<Arena*>(str->arena);
  arena->free(str);
}

string* gapil_slice_to_string(context* ctx, slice* sli) {
  DEBUG_PRINT("gapil_slice_to_string(" SLICE_FMT ")", SLICE_ARGS(sli));
  auto ptr = gapil_slice_data(ctx, sli, GAPIL_READ);
  // Trim null terminator from the string.
  if (sli->size > 0 && ((uint8_t*)ptr)[sli->size - 1] == 0) {
    sli->size--;
  }
  return gapil_make_string(ctx->arena, sli->size, ptr);
}

void gapil_string_to_slice(context* ctx, string* str, slice* out) {
  DEBUG_PRINT("gapil_string_to_slice(str: '%s')", str->data);

  auto pool = gapil_make_pool(ctx, str->length);
  auto buf = reinterpret_cast<uint8_t*>(
      gapil_resolve_pool_data(ctx, pool, 0, GAPIL_WRITE, str->length + 1));
  memcpy(buf, str->data, str->length);
  str->data[str->length] = 0;  // Null-terminate

  out->pool = pool;
  out->base = 0;
  out->root = 0;
  out->size = str->length;
  out->count = str->length;
}

string* gapil_string_concat(string* a, string* b) {
  DEBUG_PRINT("gapil_string_concat(a: '%s', b: '%s')", a->data, b->data);
  GAPID_ASSERT(a->ref_count > 0);
  GAPID_ASSERT(b->ref_count > 0);

  if (a->length == 0) {
    b->ref_count++;
    return b;
  }
  if (b->length == 0) {
    a->ref_count++;
    return a;
  }

  GAPID_ASSERT_MSG(a->arena != nullptr,
                   "string concat using string with no arena");
  GAPID_ASSERT_MSG(b->arena != nullptr,
                   "string concat using string with no arena");

  auto str = gapil_make_string(a->arena, a->length + b->length, nullptr);
  memcpy(str->data, a->data, a->length);
  memcpy(str->data + a->length, b->data, b->length);
  return str;
}

int32_t gapil_string_compare(string* a, string* b) {
  DEBUG_PRINT("gapil_string_compare(a: '%s', b: '%s')", a->data, b->data);
  if (a == b) {
    return 0;
  }
  return strncmp(reinterpret_cast<const char*>(a->data),
                 reinterpret_cast<const char*>(b->data),
                 std::max(a->length, b->length));
}

void gapil_apply_reads(context* ctx) {
  DEBUG_PRINT("gapil_apply_reads(ctx: %p)", ctx);
  GAPID_ASSERT(runtime_callbacks.apply_reads != nullptr);
  runtime_callbacks.apply_reads(ctx);
}

void gapil_apply_writes(context* ctx) {
  DEBUG_PRINT("gapil_apply_writes(ctx: %p)", ctx);
  GAPID_ASSERT(runtime_callbacks.apply_writes != nullptr);
  runtime_callbacks.apply_writes(ctx);
}

void* gapil_resolve_pool_data(context* ctx, uint64_t pool_id, uint64_t ptr,
                              gapil_data_access access, uint64_t size) {
  DEBUG_PRINT("gapil_resolve_pool_data(ctx: %p, pool: %" PRIu64
              ", ptr: 0x%" PRIx64 ", access: %d, size: 0x%" PRIx64 ")",
              ctx, pool_id, ptr, access, size);
  GAPID_ASSERT(runtime_callbacks.resolve_pool_data != nullptr);
  return runtime_callbacks.resolve_pool_data(ctx, pool_id, ptr, access, size);
}

void gapil_copy_slice(context* ctx, slice* dst, slice* src) {
  DEBUG_PRINT(
      "gapil_copy_slice(ctx: %p,\n"
      "    dst: " SLICE_FMT
      ",\n"
      "    src: " SLICE_FMT ")",
      ctx, SLICE_ARGS(dst), SLICE_ARGS(src));

  GAPID_ASSERT(runtime_callbacks.copy_slice != nullptr);
  return runtime_callbacks.copy_slice(ctx, dst, src);
}

void gapil_cstring_to_slice(context* ctx, uint64_t ptr, slice* out) {
  DEBUG_PRINT("gapil_cstring_to_slice(ctx: %p, ptr: 0x%" PRIx64 ", out: %p)",
              ctx, ptr, out);

  GAPID_ASSERT(runtime_callbacks.cstring_to_slice != nullptr);
  return runtime_callbacks.cstring_to_slice(ctx, ptr, out);
}

void gapil_store_in_database(context* ctx, void* ptr, uint64_t size,
                             uint8_t* id_out) {
  DEBUG_PRINT("gapil_store_in_database(ctx: %p, ptr: %p, size: 0x%" PRIx64
              ", id_out:  %p)",
              ctx, ptr, size, id_out);
  GAPID_ASSERT(runtime_callbacks.store_in_database != nullptr);
  runtime_callbacks.store_in_database(ctx, ptr, size, id_out);
}

uint64_t gapil_make_pool(context* ctx, uint64_t size) {
  DEBUG_PRINT("gapil_make_pool(ctx: %p, size: %" PRIu64 ")", ctx, size);
  GAPID_ASSERT(runtime_callbacks.make_pool != nullptr);
  return runtime_callbacks.make_pool(ctx, size);
}

void gapil_pool_reference(context* ctx, uint64_t pool_id) {
  DEBUG_PRINT("gapil_pool_reference(pool: %" PRIu64 ")", pool_id);
  GAPID_ASSERT(runtime_callbacks.pool_reference != nullptr);
  if (pool_id == 0) {
    GAPID_FATAL("Attempting to reference application pool")
  }
  runtime_callbacks.pool_reference(ctx, pool_id);
}

void gapil_pool_release(context* ctx, uint64_t pool_id) {
  DEBUG_PRINT("gapil_pool_release(pool: %" PRIu64 ")", pool_id);
  GAPID_ASSERT(runtime_callbacks.pool_release != nullptr);
  if (pool_id == 0) {
    GAPID_FATAL("Attempting to release application pool")
  }
  runtime_callbacks.pool_release(ctx, pool_id);
}

void gapil_call_extern(context* ctx, uint8_t* name, void* args, void* res) {
  DEBUG_PRINT("gapil_call_extern(ctx: %p, name: %s, args: %p, res: %p)", ctx,
              name, args, res);
  GAPID_ASSERT(runtime_callbacks.call_extern != nullptr);
  runtime_callbacks.call_extern(ctx, name, args, res);
}

}  // extern "C"
