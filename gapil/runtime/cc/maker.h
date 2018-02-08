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

#ifndef __GAPIL_RUNTIME_ZERO_H__
#define __GAPIL_RUNTIME_ZERO_H__

#include <type_traits>

namespace core {
class Arena;
}  // namespace core

namespace gapil {

template<typename T, bool TAKES_ARENA>
struct Maker;

template<typename T>
struct Maker<T, true> {
    template<typename ...ARGS>
    static inline T make(core::Arena* a, ARGS&&... args) { return T(a, std::forward<ARGS>(args)...); }

    template<typename ...ARGS>
    static inline void inplace_new(T* ptr, core::Arena* a, ARGS&&... args) { new(ptr) T(a, std::forward<ARGS>(args)...); }
};

template<typename T>
struct Maker<T, false> {
    template<typename ...ARGS>
    static inline T make(core::Arena*, ARGS&&... args) { return T(std::forward<ARGS>(args)...); }

    template<typename ...ARGS>
    static inline void inplace_new(T* ptr, core::Arena* a, ARGS&&... args) { new(ptr) T(std::forward<ARGS>(args)...); }
};

template<typename T, typename ...ARGS>
inline T make(core::Arena* a, ARGS&&... args) {
    return Maker<T, std::is_constructible<T, core::Arena*, ARGS...>::value>::
            make(a, std::forward<ARGS>(args)...);
}

template<typename T, typename ...ARGS>
inline void inplace_new(T* ptr, core::Arena* a, ARGS&&... args) {
    Maker<T, std::is_constructible<T, core::Arena*, ARGS...>::value>::
            inplace_new(ptr, a, std::forward<ARGS>(args)...);
}

} // namespace std

#endif  // __GAPIL_RUNTIME_ZERO_H__