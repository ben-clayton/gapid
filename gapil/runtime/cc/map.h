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

#ifndef __GAPIL_RUNTIME_MAP_H__
#define __GAPIL_RUNTIME_MAP_H__

#include "runtime.h"

namespace core {
class Arena;
}  // namespace core

namespace gapil {

template<typename K, typename V>
class Map {
private:
    struct Allocation;
public:
    struct element {
        uint64_t used;
        K first;
        V second;
    };

    using key_type = K;
    using value_type = V;

    Map(core::Arena*);
    Map(const Map<K,V>&);
    Map(Map<K, V>&&);
    ~Map();

    Map<K,V>& operator = (const Map<K,V>&);

    class iterator {
    public:
        inline iterator(const iterator& it);

        inline bool operator == (const iterator& other);

        inline bool operator != (const iterator& other);

        inline element& operator*();

        inline element* operator->();

        inline const iterator& operator++();

        inline iterator operator++(int);

    private:
        friend class Map<K, V>;
        element* elem;
        Allocation* map;

        inline iterator(element* elem, Allocation* map);
    };

    class const_iterator {
    public:
        inline const_iterator(const iterator& it);

        inline bool operator == (const const_iterator& other);

        inline bool operator != (const const_iterator& other);

        inline const element& operator*();

        inline const element* operator->();

        inline const_iterator& operator++();

        inline const_iterator operator++(int);

    private:
        friend class Map<K, V>;
        const element* elem;
        const Allocation* map;

        inline const_iterator(const element* elem, const Allocation* map);
    };

    inline uint64_t capacity() const;

    inline uint64_t count() const;

    inline const const_iterator begin() const;

    inline iterator begin();

    inline iterator end();

    inline const_iterator end() const;

    inline void erase(const K& k);

    inline void erase(const_iterator it);

    template<typename T>
    inline V& operator[](const T& key);

    inline iterator find(const K& key);

    inline const_iterator find(const K& k) const;

private:
    struct Allocation : public map_t {
        // The following methods must match those generated by the gapil compiler.
        bool contains(K);
        V*   index(K, bool);
        V    lookup(K);
        void remove(K);
        void clear();
        void reference();
        void release();

        inline const element* els() const;
        inline element* els();
    };

    Allocation* ptr;
};

////////////////////////////////////////////////////////////////////////////////
// Map<K, V>::iterator                                                        //
////////////////////////////////////////////////////////////////////////////////

template <typename K, typename V>
Map<K, V>::iterator::iterator(element* elem, Allocation* map) :
        elem(elem), map(map) {}

template <typename K, typename V>
Map<K, V>::iterator::iterator(const iterator& it) :
        elem(it.elem), map(it.map) {}

template <typename K, typename V>
bool Map<K, V>::iterator::operator==(const iterator& other) {
    return map == other.map && elem == other.elem;
}

template <typename K, typename V>
bool Map<K, V>::iterator::operator!=(const iterator& other){
    return !(*this == other);
}

template <typename K, typename V>
typename Map<K, V>::element& Map<K, V>::iterator::operator*() {
    return *elem;
}

template <typename K, typename V>
typename Map<K, V>::element* Map<K, V>::iterator::operator->() {
    return elem;
}

template <typename K, typename V>
const typename Map<K, V>::iterator& Map<K, V>::iterator::operator++() {
    size_t offset = elem - reinterpret_cast<element*>(map->elements);
    for (size_t i = offset; i < map->capacity; ++i) {
        ++elem;
        if (elem->used == GAPIL_MAP_ELEMENT_FULL) {
            break;
        }
    }
    return *this;
}

template <typename K, typename V>
typename Map<K, V>::iterator Map<K, V>::iterator::operator++(int) {
    iterator ret = *this;
    ++(*this);
    return ret;
}

////////////////////////////////////////////////////////////////////////////////
// Map<K, V>::const_iterator                                                  //
////////////////////////////////////////////////////////////////////////////////

template <typename K, typename V>
Map<K, V>::const_iterator::const_iterator(const element* elem, const Allocation* map):
    elem(elem), map(map) {}

template <typename K, typename V>
Map<K, V>::const_iterator::const_iterator(const iterator& it):
    elem(it.elem), map(it.map) {
}

template <typename K, typename V>
bool Map<K, V>::const_iterator::operator==(const const_iterator& other) {
    return map == other.map && elem == other.elem;
}

template <typename K, typename V>
bool Map<K, V>::const_iterator::operator!=(const const_iterator& other){
    return !(*this == other);
}

template <typename K, typename V>
const typename Map<K, V>::element& Map<K, V>::const_iterator::operator*() {
    return *elem;
}

template <typename K, typename V>
const typename Map<K, V>::element* Map<K, V>::const_iterator::operator->() {
    return elem;
}

template <typename K, typename V>
typename Map<K, V>::const_iterator& Map<K, V>::const_iterator::operator++() {
    size_t offset = elem - reinterpret_cast<element*>(map->elements);
    for (size_t i = offset; i < map->capacity; ++i) {
        ++elem;
        if (elem->used == GAPIL_MAP_ELEMENT_FULL) {
            break;
        }
    }
    return *this;
}

template <typename K, typename V>
typename Map<K, V>::const_iterator Map<K, V>::const_iterator::operator++(int) {
    const_iterator ret = *this;
    ++(*this);
    return ret;
}

////////////////////////////////////////////////////////////////////////////////
// Map<K, V>                                                                  //
////////////////////////////////////////////////////////////////////////////////

template <typename K, typename V>
uint64_t Map<K, V>::capacity() const {
    return ptr->capacity;
}

template <typename K, typename V>
uint64_t Map<K, V>::count() const {
    return ptr->count;
}

template <typename K, typename V>
const typename Map<K, V>::const_iterator Map<K, V>::begin() const {
    auto it = const_iterator{ptr->els(), ptr};
    for (size_t i = 0; i < ptr->capacity; ++i) {
        if (it.elem->used == GAPIL_MAP_ELEMENT_FULL) {
            break;
        }
        it.elem++;
    }
    return it;
}

template <typename K, typename V>
typename Map<K, V>::iterator Map<K, V>::begin() {
    auto it = iterator{ptr->els(), ptr};
    for (size_t i = 0; i < ptr->capacity; ++i) {
        if (it.elem->used == GAPIL_MAP_ELEMENT_FULL) {
            break;
        }
        it.elem++;
    }
    return it;
}

template <typename K, typename V>
typename Map<K, V>::iterator Map<K, V>::end() {
    return iterator{ptr->els() + capacity(), ptr};
}

template <typename K, typename V>
typename Map<K, V>::const_iterator Map<K, V>::end() const {
    return const_iterator{ptr->els() + capacity(), ptr};
}

template <typename K, typename V>
void Map<K, V>::erase(const K& k) {
    ptr->remove(k);
}

template <typename K, typename V>
void Map<K, V>::erase(const_iterator it) {
    ptr->remove(it->first);
}

template <typename K, typename V>
template <typename T>
V& Map<K, V>::operator[](const T& key) {
    V* v = ptr->index(key, true);
    return *v;
}

template <typename K, typename V>
typename Map<K, V>::iterator Map<K, V>::find(const K& key) {
    V* idx = ptr->index(key, false);
    if (idx == nullptr) {
        return end();
    }
    size_t offs =
        (reinterpret_cast<uintptr_t>(idx) - reinterpret_cast<uintptr_t>(ptr->els())) / sizeof(element);
    return iterator{ptr->els() + offs, ptr};
}

template <typename K, typename V>
typename Map<K, V>::const_iterator Map<K, V>::find(const K& k) const {
    // Sorry for the const_cast. We know that if the last element is false,
    // this wont be modified.
    const V* idx = const_cast<Map<K, V>*>(ptr)->index(k, false);
    if (idx == nullptr) {
        return end();
    }
    size_t offs =
        (reinterpret_cast<uintptr_t>(idx) - reinterpret_cast<uintptr_t>(ptr->els())) / sizeof(element);
    return const_iterator{ptr->els() + offs, ptr};
}

////////////////////////////////////////////////////////////////////////////////
// Map<K, V>::Alocation                                                       //
////////////////////////////////////////////////////////////////////////////////

template <typename K, typename V>
const typename Map<K, V>::element* Map<K, V>::Allocation::els() const {
    return reinterpret_cast<const element*>(map_t::elements);
}

template <typename K, typename V>
typename Map<K, V>::element* Map<K, V>::Allocation::els() {
    return reinterpret_cast<element*>(map_t::elements);
}

}  // namespace gapil

#endif  // __GAPIL_RUNTIME_MAP_H__