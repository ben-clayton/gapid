/*
 * Copyright (C) 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include "pool.h"
#include "spy.h"

#include "core/memory/arena/cc/arena.h"

#include "gapil/runtime/cc/encoder.h"
#include "gapil/runtime/cc/runtime.h"

#include "gapii/cc/call_observer.h"
#include "gapii/cc/pack_encoder.h"

#if 0
#define DEBUG_PRINT(...) GAPID_DEBUG(__VA_ARGS__)
#else
#define DEBUG_PRINT(...)
#endif

extern "C" {

int64_t gapil_encode_type(context* ctx, uint8_t* name, uint32_t desc_size,
                          void* desc) {
  DEBUG_PRINT("gapil_encode_type(%p, %s, %d, %p)", ctx, name, desc_size, desc);
  auto cb = static_cast<gapii::CallObserver*>(ctx);
  auto e = cb->encoder();
  auto res = e->type(reinterpret_cast<const char*>(name), desc_size, desc);
  auto id = static_cast<int64_t>(res.first);
  auto isnew = res.second;
  return isnew ? id : -id;
}

void* gapil_encode_object(context* ctx, uint8_t is_group, uint32_t type,
                          uint32_t data_size, void* data) {
  DEBUG_PRINT("gapil_encode_object(%p, %s, %d, %d, %p)", ctx,
              is_group ? "true" : "false", type, data_size, data);
  auto cb = static_cast<gapii::CallObserver*>(ctx);
  auto e = cb->encoder();
  if (is_group) {
    return e->group(type, data_size, data);
  }
  e->object(type, data_size, data);
  return nullptr;
}

void gapil_slice_encoded(context* ctx, slice_t* slice) {
  DEBUG_PRINT("gapil_on_encode_slice(%p, %p)", ctx, slice);
  auto cb = static_cast<gapii::CallObserver*>(ctx);
  cb->slice_encoded(slice);
}

int64_t gapil_encode_backref(context* ctx, void* object) {
  auto cb = static_cast<gapii::CallObserver*>(ctx);
  auto res = cb->reference_id(object);
  auto id = static_cast<int64_t>(res.first);
  auto isnew = res.second;
  DEBUG_PRINT("gapil_encode_backref(%p, %p) -> new: %s id: %d", ctx, object,
              isnew ? "true" : "false", int(id));
  return isnew ? id : -id;
}

void* resolve_pool_data(context* ctx, pool* pool, uint64_t ptr,
                        gapil_data_access access, uint64_t size) {
  return (pool == nullptr)
             ? reinterpret_cast<void*>(static_cast<uintptr_t>(ptr))
             : &pool->buffer[ptr];
}

pool_t* make_pool(context* ctx, uint64_t size) {
  auto cb = static_cast<gapii::CallObserver*>(ctx);
  auto arena = reinterpret_cast<core::Arena*>(ctx->arena);
  auto pool = arena->create<pool_t>();
  pool->ref_count = 1;
  pool->id = cb->allocate_pool_id();
  pool->size = size;
  pool->buffer = reinterpret_cast<uint8_t*>(arena->allocate(size, 16));
  pool->arena = ctx->arena;
  return pool;
}

void pool_reference(pool_t* pool) {
  GAPID_ASSERT_MSG(pool->ref_count > 0,
                   "Attempting to reference pool with no references");
  pool->ref_count++;
}

void pool_release(pool_t* pool) {
  GAPID_ASSERT_MSG(pool->ref_count > 0,
                   "Attempting to reference pool with no references");
  pool->ref_count--;
  if (pool->ref_count == 0) {
    auto arena = reinterpret_cast<core::Arena*>(pool->arena);
    arena->free(pool->buffer);
    arena->free(pool);
  }
}

uint64_t pool_id(pool_t* pool) { return pool->id; }

}  // extern "C"

namespace gapii {

void Spy::register_runtime_callbacks() {
  gapil_runtime_callbacks cb = {0};
  cb.resolve_pool_data = &resolve_pool_data;
  cb.make_pool = &make_pool;
  cb.pool_reference = &pool_reference;
  cb.pool_release = &pool_release;
  cb.pool_id = &pool_id;
  gapil_set_runtime_callbacks(&cb);
}

}  // namespace gapii