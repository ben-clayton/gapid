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

#include "memory.h"

#include "core/cc/assert.h"
#include "core/cc/interval_list.h"
#include "core/memory/arena/cc/arena.h"

#define __STDC_FORMAT_MACROS
#include <inttypes.h>

#include <cstring>

#if 1
#define DEBUG_PRINT(...) GAPID_WARNING(__VA_ARGS__)
#else
#define DEBUG_PRINT(...)
#endif

#define DATA_FMT \
  "[ps: %" PRIu64 ", pe: 0x%" PRIx64 ", ds: 0x%" PRIx64 ", de: 0x%" PRIx64 "]"
#define DATA_ARGS(data) \
  (data).data_start, (data).data_end, (data).pool_start, (data).pool_end

struct Data {
  typedef uint64_t interval_unit_type;

  // Interval compilance
  inline uint64_t start() const { return data_start; }
  inline uint64_t end() const { return data_end; }
  inline void adjust(uint64_t start, uint64_t end) {
    data_start = start;
    data_end = end;
  }
  inline uint64_t data_size() const { return data_end - data_start; }

  uint8_t* get() const;
  void get(void* out, uint64_t offset, uint64_t size) const;

  enum class Kind {
    BYTES,
    RESOURCE,
  };

  uint64_t pool_start;
  uint64_t pool_end;
  uint64_t data_start;
  uint64_t data_end;
  uint8_t* data;
  Kind kind;
};

uint8_t* Data::get() const {
  switch (kind) {
    case Kind::BYTES: {
      auto offset = data_start - pool_start;
      return data + offset;
    }
    case Kind::RESOURCE:
      // TODO
      return nullptr;
    default:
      GAPID_ASSERT_MSG(false, "Unknown data kind");
      return nullptr;
  }
}

void Data::get(void* out, uint64_t offset, uint64_t size) const {
  GAPID_ASSERT(size + offset <= data_size());
  memcpy(out, get() + offset, size);
}

class Pool {
 public:
  void* read(core::Arena* arena, uint64_t addr, uint64_t size,
             GAPIL_BOOL* free_ptr);
  void write(core::Arena* arena, size_t base, size_t size, const void* data);
  void copy(core::Arena* arena, Pool* src_pool, size_t dst_base,
            size_t src_base, size_t size);

 private:
  core::CustomIntervalList<Data> writes_;
};

void* Pool::read(core::Arena* arena, uint64_t addr, uint64_t size,
                 GAPIL_BOOL* free_ptr) {
  DEBUG_PRINT("Pool::read(arena: %p, addr: 0x%" PRIx64 ", size: 0x%" PRIx64
              ", free_ptr: %p",
              arena, addr, size, free_ptr);

  auto intervals = writes_.intersect(addr, addr + size);
  if (intervals.size() == 1) {
    auto data = intervals.begin();
    if (addr >= data->data_start && size <= data->data_size()) {
      auto offset = addr - data->data_start;
      DEBUG_PRINT("    single intersection: " DATA_FMT " offset: %d",
                  DATA_ARGS(*data), int(offset));
      *free_ptr = GAPIL_FALSE;
      return reinterpret_cast<uint8_t*>(data->get()) + offset;
    }
  }

  DEBUG_PRINT("    %d intersections", int(intervals.size()));
  uint8_t* out = reinterpret_cast<uint8_t*>(arena->allocate(size, 8));
  *free_ptr = GAPIL_TRUE;
  memset(out, 0, size);
  for (auto& data : intervals) {
    DEBUG_PRINT("    interval: " DATA_FMT, DATA_ARGS(data));
    auto dst_offset = (data.data_start > addr) ? data.data_start - addr : 0;
    auto src_offset = (addr > data.data_start) ? addr - data.data_start : 0;
    auto dst_size = size - dst_offset;
    auto src_size = data.data_size() - src_offset;
    auto size = std::min(dst_size, src_size);
    DEBUG_PRINT("    get(out + %d, %d, %d): ", int(dst_offset), int(src_offset),
                int(size));
    data.get(out + dst_offset, src_offset, size);
  }
  return out;
}

void Pool::write(core::Arena* arena, size_t base, size_t size,
                 const void* data) {
  DEBUG_PRINT("Pool::write(arena: %p, base: 0x%" PRIx64 ", size: 0x%" PRIx64
              ", data: %p",
              arena, base, size, data);

  auto start = base;
  auto end = base + size;
  auto alloc = arena->allocate(size, 8);
  memcpy(alloc, data, size);
  writes_.replace(Data{
      .pool_start = start,
      .pool_end = end,
      .data_start = start,
      .data_end = end,
      .data = reinterpret_cast<uint8_t*>(alloc),
      .kind = Data::Kind::BYTES,
  });
}

void Pool::copy(core::Arena* arena, Pool* src_pool, size_t dst_base,
                size_t src_base, size_t size) {
  auto intervals = src_pool->writes_.intersect(src_base, src_base + size);
  auto start = src_base;
  auto end = src_base + size;
  for (auto data : intervals) {
    data.data_start = std::max(data.data_start, start);
    data.data_end = std::min(data.data_end, end);
    writes_.replace(data);
  }
}

class Memory {
 public:
  Memory(core::Arena*);

  void* read(pool_id pool, uint64_t addr, uint64_t size, GAPIL_BOOL* free_ptr);
  void write(pool_id pool, uint64_t addr, uint64_t size, const void* data);
  void copy(slice* dst, slice* src);
  pool_id new_pool();

 private:
  Pool* get_pool(pool_id id);

  core::Arena* arena_;
  pool_id next_pool_id;
  std::unordered_map<pool_id, Pool*> pools_;
};

Memory::Memory(core::Arena* a) : arena_(a), next_pool_id(1) {}

void* Memory::read(pool_id pool, uint64_t addr, uint64_t size,
                   GAPIL_BOOL* free_ptr) {
  auto p = get_pool(pool);
  return p->read(arena_, addr, size, free_ptr);
}

void Memory::write(pool_id pool, uint64_t addr, uint64_t size,
                   const void* data) {
  auto p = get_pool(pool);
  return p->write(arena_, addr, size, data);
}

void Memory::copy(slice* dst, slice* src) {
  auto d = get_pool(dst->pool);
  auto s = get_pool(src->pool);
  auto size = std::min(dst->size, src->size);
  d->copy(arena_, s, dst->base, src->base, size);
}

pool_id Memory::new_pool() {
  auto id = next_pool_id++;
  pools_[id] = arena_->create<Pool>();
  return id;
}

Pool* Memory::get_pool(pool_id id) {
  auto it = pools_.find(id);
  GAPID_ASSERT_MSG(it != pools_.end(), "Pool %d does not exist", int(id));
  return it->second;
}

extern "C" {

memory* memory_create(arena* a) {
  auto arena = reinterpret_cast<core::Arena*>(a);
  return reinterpret_cast<memory*>(new Memory(arena));
}

void memory_destroy(memory* mem) {
  auto m = reinterpret_cast<Memory*>(mem);
  delete m;
}

void* memory_read(memory* mem, pool_id pool, uint64_t addr, uint64_t size,
                  GAPIL_BOOL* free_ptr) {
  auto m = reinterpret_cast<Memory*>(mem);
  return m->read(pool, addr, size, free_ptr);
}

void memory_write(memory* mem, pool_id pool, uint64_t addr, uint64_t size,
                  const void* data) {
  auto m = reinterpret_cast<Memory*>(mem);
  return m->write(pool, addr, size, data);
}

void memory_copy(memory* mem, slice* dst, slice* src) {
  auto m = reinterpret_cast<Memory*>(mem);
  return m->copy(dst, src);
}

pool_id memory_new_pool(memory* mem) {
  auto m = reinterpret_cast<Memory*>(mem);
  return m->new_pool();
}

}  // extern "C"