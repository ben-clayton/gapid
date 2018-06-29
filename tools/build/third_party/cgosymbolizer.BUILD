# Copyright (C) 2018 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("@io_bazel_rules_go//go:def.bzl", "go_library")

cc_library(
    name = "cc",
    srcs = [
        "dwarf.c",
        "elf.c",
        "fileline.c",
        "mmap.c",
        "mmapio.c",
        "posix.c",
        "sort.c",
        "state.c",
        "symbolizer.c",
        "traceback.c",
        "internal.h",
        "backtrace.h",
    ],
    deps = [
        "@gapid//tools/build/third_party:cgosymbolizer-config",
        "@libbacktrace",
    ],
    linkopts = select({
        "@gapid//tools/build:linux": ["-ldl"],
        "@gapid//tools/build:darwin": ["-ldl"],
        "@gapid//tools/build:windows": [],
    }),
    visibility = ["//visibility:private"],
)

go_library(
    name = "go_default_library",
    srcs = glob(["*.go"]),
    cdeps = [":cc"],
    cgo = True,
    importpath = "github.com/ianlancetaylor/cgosymbolizer",
    visibility = ["//visibility:public"],
)
