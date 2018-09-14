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

#ifndef GAPIS_MEMORY_POOL_H
#define GAPIS_MEMORY_POOL_H

#include "gapil/runtime/cc/runtime.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct memory_t memory;
typedef uint64_t pool_id;

memory* memory_create(arena*);
void memory_destroy(memory*);

void* memory_read(memory*, pool_id, uint64_t addr, uint64_t size,
                  GAPIL_BOOL* free_ptr);
void memory_write(memory*, pool_id, uint64_t addr, uint64_t size,
                  const void* data);
void memory_copy(memory*, slice* dst, slice* src);
pool_id memory_new_pool(memory*);

#ifdef __cplusplus
}  // extern "C"
#endif

#endif  // GAPIS_MEMORY_POOL_H