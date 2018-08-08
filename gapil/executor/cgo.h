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

#include "gapil/runtime/cc/runtime.h"

void applyReadsCgo(context*);
void applyWritesCgo(context*);
void* resolvePoolDataCgo(context*, uint64_t pool_id, uint64_t ptr,
                         gapil_data_access, uint64_t size);
void callExternCgo(context*, uint8_t* name, void* args, void* res);
void copySliceCgo(context*, slice* dst, slice* src);
void cstringToSliceCgo(context*, uint64_t ptr, slice* out);
void storeInDatabaseCgo(context* ctx, void* ptr, uint64_t size,
                        uint8_t* id_out);
uint64_t makePoolCgo(context*, uint64_t size);
void poolReferenceCgo(context*, uint64_t pool_id);
void poolReleaseCgo(context*, uint64_t pool_id);