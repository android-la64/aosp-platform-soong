// Copyright 2021 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bp2build

import (
	"fmt"
	"testing"

	"android/soong/aidl_library"
	"android/soong/android"
	"android/soong/cc"
)

const (
	// See cc/testing.go for more context
	soongCcLibraryPreamble = `
cc_defaults {
    name: "linux_bionic_supported",
}
`

	soongCcVersionLibBpPath = "build/soong/cc/libbuildversion/Android.bp"
	soongCcVersionLibBp     = `
cc_library_static {
	name: "libbuildversion",
}
`

	soongCcProtoLibraries = `
cc_library {
	name: "libprotobuf-cpp-lite",
}

cc_library {
	name: "libprotobuf-cpp-full",
}`

	soongCcProtoPreamble = soongCcLibraryPreamble + soongCcProtoLibraries
)

func runCcLibraryTestCase(t *testing.T, tc Bp2buildTestCase) {
	t.Helper()
	tc.StubbedBuildDefinitions = append(tc.StubbedBuildDefinitions, "libprotobuf-cpp-lite", "libprotobuf-cpp-full")
	RunBp2BuildTestCase(t, registerCcLibraryModuleTypes, tc)
}

func registerCcLibraryModuleTypes(ctx android.RegistrationContext) {
	cc.RegisterCCBuildComponents(ctx)
	ctx.RegisterModuleType("filegroup", android.FileGroupFactory)
	ctx.RegisterModuleType("cc_library_static", cc.LibraryStaticFactory)
	ctx.RegisterModuleType("cc_prebuilt_library_static", cc.PrebuiltStaticLibraryFactory)
	ctx.RegisterModuleType("cc_library_headers", cc.LibraryHeaderFactory)
	ctx.RegisterModuleType("aidl_library", aidl_library.AidlLibraryFactory)
	ctx.RegisterModuleType("ndk_library", cc.NdkLibraryFactory)
}

func TestCcLibrarySimple(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - simple example",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"//build/soong/cc/libbuildversion:libbuildversion", "some-headers"},
		Filesystem: map[string]string{
			soongCcVersionLibBpPath: soongCcVersionLibBp,
			"android.cpp":           "",
			"bionic.cpp":            "",
			"darwin.cpp":            "",
			// Refer to cc.headerExts for the supported header extensions in Soong.
			"header.h":         "",
			"header.hh":        "",
			"header.hpp":       "",
			"header.hxx":       "",
			"header.h++":       "",
			"header.inl":       "",
			"header.inc":       "",
			"header.ipp":       "",
			"header.h.generic": "",
			"impl.cpp":         "",
			"linux.cpp":        "",
			"x86.cpp":          "",
			"x86_64.cpp":       "",
			"foo-dir/a.h":      "",
		},
		Blueprint: soongCcLibraryPreamble +
			simpleModule("cc_library_headers", "some-headers") + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    cflags: ["-Wall"],
    header_libs: ["some-headers"],
    export_include_dirs: ["foo-dir"],
    ldflags: ["-Wl,--exclude-libs=bar.a"],
    arch: {
        x86: {
            ldflags: ["-Wl,--exclude-libs=baz.a"],
            srcs: ["x86.cpp"],
        },
        x86_64: {
            ldflags: ["-Wl,--exclude-libs=qux.a"],
            srcs: ["x86_64.cpp"],
        },
    },
    target: {
        android: {
            srcs: ["android.cpp"],
        },
        linux_glibc: {
            srcs: ["linux.cpp"],
        },
        darwin: {
            srcs: ["darwin.cpp"],
        },
        bionic: {
          srcs: ["bionic.cpp"]
        },
    },
    include_build_directory: false,
    sdk_version: "current",
    min_sdk_version: "29",
    use_version_lib: true,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"copts":               `["-Wall"]`,
			"export_includes":     `["foo-dir"]`,
			"implementation_deps": `[":some-headers"]`,
			"linkopts": `["-Wl,--exclude-libs=bar.a"] + select({
        "//build/bazel_common_rules/platforms/arch:x86": ["-Wl,--exclude-libs=baz.a"],
        "//build/bazel_common_rules/platforms/arch:x86_64": ["-Wl,--exclude-libs=qux.a"],
        "//conditions:default": [],
    })`,
			"srcs": `["impl.cpp"] + select({
        "//build/bazel_common_rules/platforms/arch:x86": ["x86.cpp"],
        "//build/bazel_common_rules/platforms/arch:x86_64": ["x86_64.cpp"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": [
            "bionic.cpp",
            "android.cpp",
        ],
        "//build/bazel_common_rules/platforms/os:darwin": ["darwin.cpp"],
        "//build/bazel_common_rules/platforms/os:linux_bionic": ["bionic.cpp"],
        "//build/bazel_common_rules/platforms/os:linux_glibc": ["linux.cpp"],
        "//conditions:default": [],
    })`,
			"sdk_version":        `"current"`,
			"min_sdk_version":    `"29"`,
			"use_version_lib":    `True`,
			"whole_archive_deps": `["//build/soong/cc/libbuildversion:libbuildversion"]`,
			"deps": `select({
        "//build/bazel/rules/apex:unbundled_app": ["//build/bazel/rules/cc:ndk_sysroot"],
        "//conditions:default": [],
    })`,
		}),
	})
}

func TestCcLibraryTrimmedLdAndroid(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - trimmed example of //bionic/linker:ld-android",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"libc_headers"},
		Filesystem: map[string]string{
			"ld-android.cpp":           "",
			"linked_list.h":            "",
			"linker.h":                 "",
			"linker_block_allocator.h": "",
			"linker_cfi.h":             "",
		},
		Blueprint: soongCcLibraryPreamble +
			simpleModule("cc_library_headers", "libc_headers") + `
cc_library {
    name: "fake-ld-android",
    srcs: ["ld_android.cpp"],
    cflags: [
        "-Wall",
        "-Wextra",
        "-Wunused",
        "-Werror",
    ],
    header_libs: ["libc_headers"],
    ldflags: [
        "-Wl,--exclude-libs=libgcc.a",
        "-Wl,--exclude-libs=libgcc_stripped.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-arm-android.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-aarch64-android.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-i686-android.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-x86_64-android.a",
    ],
    arch: {
        x86: {
            ldflags: ["-Wl,--exclude-libs=libgcc_eh.a"],
        },
        x86_64: {
            ldflags: ["-Wl,--exclude-libs=libgcc_eh.a"],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("fake-ld-android", AttrNameToString{
			"srcs": `["ld_android.cpp"]`,
			"copts": `[
        "-Wall",
        "-Wextra",
        "-Wunused",
        "-Werror",
    ]`,
			"implementation_deps": `[":libc_headers"]`,
			"linkopts": `[
        "-Wl,--exclude-libs=libgcc.a",
        "-Wl,--exclude-libs=libgcc_stripped.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-arm-android.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-aarch64-android.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-i686-android.a",
        "-Wl,--exclude-libs=libclang_rt.builtins-x86_64-android.a",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:x86": ["-Wl,--exclude-libs=libgcc_eh.a"],
        "//build/bazel_common_rules/platforms/arch:x86_64": ["-Wl,--exclude-libs=libgcc_eh.a"],
        "//conditions:default": [],
    })`,
		}),
	})
}

func TestCcLibraryExcludeSrcs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library exclude_srcs - trimmed example of //external/arm-optimized-routines:libarm-optimized-routines-math",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "external",
		Filesystem: map[string]string{
			"external/math/cosf.c":      "",
			"external/math/erf.c":       "",
			"external/math/erf_data.c":  "",
			"external/math/erff.c":      "",
			"external/math/erff_data.c": "",
			"external/Android.bp": `
cc_library {
    name: "fake-libarm-optimized-routines-math",
    exclude_srcs: [
        // Provided by:
        // bionic/libm/upstream-freebsd/lib/msun/src/s_erf.c
        // bionic/libm/upstream-freebsd/lib/msun/src/s_erff.c
        "math/erf.c",
        "math/erf_data.c",
        "math/erff.c",
        "math/erff_data.c",
    ],
    srcs: [
        "math/*.c",
    ],
    // arch-specific settings
    arch: {
        arm64: {
            cflags: [
                "-DHAVE_FAST_FMA=1",
            ],
        },
    },
    bazel_module: { bp2build_available: true },
}
`,
		},
		Blueprint: soongCcLibraryPreamble,
		ExpectedBazelTargets: makeCcLibraryTargets("fake-libarm-optimized-routines-math", AttrNameToString{
			"copts": `select({
        "//build/bazel_common_rules/platforms/arch:arm64": ["-DHAVE_FAST_FMA=1"],
        "//conditions:default": [],
    })`,
			"local_includes": `["."]`,
			"srcs_c":         `["math/cosf.c"]`,
		}),
	})
}

func TestCcLibrarySharedStaticProps(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library shared/static props",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"both.cpp":       "",
			"sharedonly.cpp": "",
			"staticonly.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "a",
    srcs: ["both.cpp"],
    cflags: ["bothflag"],
    shared_libs: ["shared_dep_for_both"],
    static_libs: ["static_dep_for_both", "whole_and_static_lib_for_both"],
    whole_static_libs: ["whole_static_lib_for_both", "whole_and_static_lib_for_both"],
    static: {
        srcs: ["staticonly.cpp"],
        cflags: ["staticflag"],
        shared_libs: ["shared_dep_for_static"],
        static_libs: ["static_dep_for_static"],
        whole_static_libs: ["whole_static_lib_for_static"],
    },
    shared: {
        srcs: ["sharedonly.cpp"],
        cflags: ["sharedflag"],
        shared_libs: ["shared_dep_for_shared"],
        static_libs: ["static_dep_for_shared"],
        whole_static_libs: ["whole_static_lib_for_shared"],
    },
    include_build_directory: false,
}

cc_library_static {
    name: "static_dep_for_shared",
}

cc_library_static {
    name: "static_dep_for_static",
}

cc_library_static {
    name: "static_dep_for_both",
}

cc_library_static {
    name: "whole_static_lib_for_shared",
}

cc_library_static {
    name: "whole_static_lib_for_static",
}

cc_library_static {
    name: "whole_static_lib_for_both",
}

cc_library_static {
    name: "whole_and_static_lib_for_both",
}

cc_library {
    name: "shared_dep_for_shared",
}

cc_library {
    name: "shared_dep_for_static",
}

cc_library {
    name: "shared_dep_for_both",
}
`,
		StubbedBuildDefinitions: []string{"static_dep_for_shared", "static_dep_for_static",
			"static_dep_for_both", "whole_static_lib_for_shared", "whole_static_lib_for_static",
			"whole_static_lib_for_both", "whole_and_static_lib_for_both", "shared_dep_for_shared",
			"shared_dep_for_static", "shared_dep_for_both",
		},
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"copts": `[
        "bothflag",
        "staticflag",
    ]`,
				"implementation_deps": `[
        ":static_dep_for_both",
        ":static_dep_for_static",
    ]`,
				"implementation_dynamic_deps": `[
        ":shared_dep_for_both",
        ":shared_dep_for_static",
    ]`,
				"srcs": `[
        "both.cpp",
        "staticonly.cpp",
    ]`,
				"whole_archive_deps": `[
        ":whole_static_lib_for_both",
        ":whole_and_static_lib_for_both",
        ":whole_static_lib_for_static",
    ]`}),
			MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"copts": `[
        "bothflag",
        "sharedflag",
    ]`,
				"implementation_deps": `[
        ":static_dep_for_both",
        ":static_dep_for_shared",
    ]`,
				"implementation_dynamic_deps": `[
        ":shared_dep_for_both",
        ":shared_dep_for_shared",
    ]`,
				"srcs": `[
        "both.cpp",
        "sharedonly.cpp",
    ]`,
				"whole_archive_deps": `[
        ":whole_static_lib_for_both",
        ":whole_and_static_lib_for_both",
        ":whole_static_lib_for_shared",
    ]`,
			}),
		},
	})
}

func TestCcLibraryDeps(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library shared/static props",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"both.cpp":       "",
			"sharedonly.cpp": "",
			"staticonly.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "a",
    srcs: ["both.cpp"],
    cflags: ["bothflag"],
    shared_libs: ["implementation_shared_dep_for_both", "shared_dep_for_both"],
    export_shared_lib_headers: ["shared_dep_for_both"],
    static_libs: ["implementation_static_dep_for_both", "static_dep_for_both"],
    export_static_lib_headers: ["static_dep_for_both", "whole_static_dep_for_both"],
    whole_static_libs: ["not_explicitly_exported_whole_static_dep_for_both", "whole_static_dep_for_both"],
    static: {
        srcs: ["staticonly.cpp"],
        cflags: ["staticflag"],
        shared_libs: ["implementation_shared_dep_for_static", "shared_dep_for_static"],
        export_shared_lib_headers: ["shared_dep_for_static"],
        static_libs: ["implementation_static_dep_for_static", "static_dep_for_static"],
        export_static_lib_headers: ["static_dep_for_static", "whole_static_dep_for_static"],
        whole_static_libs: ["not_explicitly_exported_whole_static_dep_for_static", "whole_static_dep_for_static"],
    },
    shared: {
        srcs: ["sharedonly.cpp"],
        cflags: ["sharedflag"],
        shared_libs: ["implementation_shared_dep_for_shared", "shared_dep_for_shared"],
        export_shared_lib_headers: ["shared_dep_for_shared"],
        static_libs: ["implementation_static_dep_for_shared", "static_dep_for_shared"],
        export_static_lib_headers: ["static_dep_for_shared", "whole_static_dep_for_shared"],
        whole_static_libs: ["not_explicitly_exported_whole_static_dep_for_shared", "whole_static_dep_for_shared"],
    },
    include_build_directory: false,
}
` + simpleModule("cc_library_static", "static_dep_for_shared") +
			simpleModule("cc_library_static", "implementation_static_dep_for_shared") +
			simpleModule("cc_library_static", "static_dep_for_static") +
			simpleModule("cc_library_static", "implementation_static_dep_for_static") +
			simpleModule("cc_library_static", "static_dep_for_both") +
			simpleModule("cc_library_static", "implementation_static_dep_for_both") +
			simpleModule("cc_library_static", "whole_static_dep_for_shared") +
			simpleModule("cc_library_static", "not_explicitly_exported_whole_static_dep_for_shared") +
			simpleModule("cc_library_static", "whole_static_dep_for_static") +
			simpleModule("cc_library_static", "not_explicitly_exported_whole_static_dep_for_static") +
			simpleModule("cc_library_static", "whole_static_dep_for_both") +
			simpleModule("cc_library_static", "not_explicitly_exported_whole_static_dep_for_both") +
			simpleModule("cc_library", "shared_dep_for_shared") +
			simpleModule("cc_library", "implementation_shared_dep_for_shared") +
			simpleModule("cc_library", "shared_dep_for_static") +
			simpleModule("cc_library", "implementation_shared_dep_for_static") +
			simpleModule("cc_library", "shared_dep_for_both") +
			simpleModule("cc_library", "implementation_shared_dep_for_both"),
		StubbedBuildDefinitions: []string{"static_dep_for_shared", "implementation_static_dep_for_shared",
			"static_dep_for_static", "implementation_static_dep_for_static", "static_dep_for_both",
			"implementation_static_dep_for_both", "whole_static_dep_for_shared",
			"not_explicitly_exported_whole_static_dep_for_shared", "whole_static_dep_for_static",
			"not_explicitly_exported_whole_static_dep_for_static", "whole_static_dep_for_both",
			"not_explicitly_exported_whole_static_dep_for_both", "shared_dep_for_shared",
			"implementation_shared_dep_for_shared", "shared_dep_for_static",
			"implementation_shared_dep_for_static", "shared_dep_for_both",
			"implementation_shared_dep_for_both",
		},
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"copts": `[
        "bothflag",
        "staticflag",
    ]`,
				"deps": `[
        ":static_dep_for_both",
        ":static_dep_for_static",
    ]`,
				"dynamic_deps": `[
        ":shared_dep_for_both",
        ":shared_dep_for_static",
    ]`,
				"implementation_deps": `[
        ":implementation_static_dep_for_both",
        ":implementation_static_dep_for_static",
    ]`,
				"implementation_dynamic_deps": `[
        ":implementation_shared_dep_for_both",
        ":implementation_shared_dep_for_static",
    ]`,
				"srcs": `[
        "both.cpp",
        "staticonly.cpp",
    ]`,
				"whole_archive_deps": `[
        ":not_explicitly_exported_whole_static_dep_for_both",
        ":whole_static_dep_for_both",
        ":not_explicitly_exported_whole_static_dep_for_static",
        ":whole_static_dep_for_static",
    ]`,
			}),
			MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"copts": `[
        "bothflag",
        "sharedflag",
    ]`,
				"deps": `[
        ":static_dep_for_both",
        ":static_dep_for_shared",
    ]`,
				"dynamic_deps": `[
        ":shared_dep_for_both",
        ":shared_dep_for_shared",
    ]`,
				"implementation_deps": `[
        ":implementation_static_dep_for_both",
        ":implementation_static_dep_for_shared",
    ]`,
				"implementation_dynamic_deps": `[
        ":implementation_shared_dep_for_both",
        ":implementation_shared_dep_for_shared",
    ]`,
				"srcs": `[
        "both.cpp",
        "sharedonly.cpp",
    ]`,
				"whole_archive_deps": `[
        ":not_explicitly_exported_whole_static_dep_for_both",
        ":whole_static_dep_for_both",
        ":not_explicitly_exported_whole_static_dep_for_shared",
        ":whole_static_dep_for_shared",
    ]`,
			})},
	},
	)
}

func TestCcLibraryWholeStaticLibsAlwaysLink(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "foo/bar",
		StubbedBuildDefinitions: []string{"//foo/bar:whole_static_lib_for_shared", "//foo/bar:whole_static_lib_for_static",
			"//foo/bar:whole_static_lib_for_both"},
		Filesystem: map[string]string{
			"foo/bar/Android.bp": `
cc_library {
    name: "a",
    whole_static_libs: ["whole_static_lib_for_both"],
    static: {
        whole_static_libs: ["whole_static_lib_for_static"],
    },
    shared: {
        whole_static_libs: ["whole_static_lib_for_shared"],
    },
    bazel_module: { bp2build_available: true },
    include_build_directory: false,
}

cc_prebuilt_library_static { name: "whole_static_lib_for_shared" }

cc_prebuilt_library_static { name: "whole_static_lib_for_static" }

cc_prebuilt_library_static { name: "whole_static_lib_for_both" }
`,
		},
		Blueprint: soongCcLibraryPreamble,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"whole_archive_deps": `[
        ":whole_static_lib_for_both_alwayslink",
        ":whole_static_lib_for_static_alwayslink",
    ]`,
			}),
			MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"whole_archive_deps": `[
        ":whole_static_lib_for_both_alwayslink",
        ":whole_static_lib_for_shared_alwayslink",
    ]`,
			}),
		},
	},
	)
}

func TestCcLibrarySharedStaticPropsInArch(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library shared/static props in arch",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "foo/bar",
		Filesystem: map[string]string{
			"foo/bar/arm.cpp":        "",
			"foo/bar/x86.cpp":        "",
			"foo/bar/sharedonly.cpp": "",
			"foo/bar/staticonly.cpp": "",
			"foo/bar/Android.bp": `
cc_library {
    name: "a",
    arch: {
        arm: {
            shared: {
                srcs: ["arm_shared.cpp"],
                cflags: ["-DARM_SHARED"],
                static_libs: ["arm_static_dep_for_shared"],
                whole_static_libs: ["arm_whole_static_dep_for_shared"],
                shared_libs: ["arm_shared_dep_for_shared"],
            },
        },
        x86: {
            static: {
                srcs: ["x86_static.cpp"],
                cflags: ["-DX86_STATIC"],
                static_libs: ["x86_dep_for_static"],
            },
        },
    },
    target: {
        android: {
            shared: {
                srcs: ["android_shared.cpp"],
                cflags: ["-DANDROID_SHARED"],
                static_libs: ["android_dep_for_shared"],
            },
        },
        android_arm: {
            shared: {
                cflags: ["-DANDROID_ARM_SHARED"],
            },
        },
    },
    srcs: ["both.cpp"],
    cflags: ["bothflag"],
    static_libs: ["static_dep_for_both"],
    static: {
        srcs: ["staticonly.cpp"],
        cflags: ["staticflag"],
        static_libs: ["static_dep_for_static"],
    },
    shared: {
        srcs: ["sharedonly.cpp"],
        cflags: ["sharedflag"],
        static_libs: ["static_dep_for_shared"],
    },
    bazel_module: { bp2build_available: true },
}

cc_library_static { name: "static_dep_for_shared" }
cc_library_static { name: "static_dep_for_static" }
cc_library_static { name: "static_dep_for_both" }

cc_library_static { name: "arm_static_dep_for_shared" }
cc_library_static { name: "arm_whole_static_dep_for_shared" }
cc_library_static { name: "arm_shared_dep_for_shared" }

cc_library_static { name: "x86_dep_for_static" }

cc_library_static { name: "android_dep_for_shared" }
`,
		},
		StubbedBuildDefinitions: []string{"//foo/bar:static_dep_for_shared", "//foo/bar:static_dep_for_static",
			"//foo/bar:static_dep_for_both", "//foo/bar:arm_static_dep_for_shared", "//foo/bar:arm_whole_static_dep_for_shared",
			"//foo/bar:arm_shared_dep_for_shared", "//foo/bar:x86_dep_for_static", "//foo/bar:android_dep_for_shared",
		},
		Blueprint: soongCcLibraryPreamble,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"copts": `[
        "bothflag",
        "staticflag",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:x86": ["-DX86_STATIC"],
        "//conditions:default": [],
    })`,
				"implementation_deps": `[
        ":static_dep_for_both",
        ":static_dep_for_static",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:x86": [":x86_dep_for_static"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
				"srcs": `[
        "both.cpp",
        "staticonly.cpp",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:x86": ["x86_static.cpp"],
        "//conditions:default": [],
    })`,
			}),
			MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"copts": `[
        "bothflag",
        "sharedflag",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:arm": ["-DARM_SHARED"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": ["-DANDROID_SHARED"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os_arch:android_arm": ["-DANDROID_ARM_SHARED"],
        "//conditions:default": [],
    })`,
				"implementation_deps": `[
        ":static_dep_for_both",
        ":static_dep_for_shared",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:arm": [":arm_static_dep_for_shared"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": [":android_dep_for_shared"],
        "//conditions:default": [],
    })`,
				"implementation_dynamic_deps": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [":arm_shared_dep_for_shared"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
				"srcs": `[
        "both.cpp",
        "sharedonly.cpp",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:arm": ["arm_shared.cpp"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": ["android_shared.cpp"],
        "//conditions:default": [],
    })`,
				"whole_archive_deps": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [":arm_whole_static_dep_for_shared"],
        "//conditions:default": [],
    })`,
			}),
		},
	},
	)
}

func TestCcLibrarySharedStaticPropsWithMixedSources(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library shared/static props with c/cpp/s mixed sources",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "foo/bar",
		StubbedBuildDefinitions:    []string{"//foo/bar:shared_filegroup", "//foo/bar:static_filegroup", "//foo/bar:both_filegroup"},
		Filesystem: map[string]string{
			"foo/bar/both_source.cpp":   "",
			"foo/bar/both_source.cc":    "",
			"foo/bar/both_source.c":     "",
			"foo/bar/both_source.s":     "",
			"foo/bar/both_source.S":     "",
			"foo/bar/shared_source.cpp": "",
			"foo/bar/shared_source.cc":  "",
			"foo/bar/shared_source.c":   "",
			"foo/bar/shared_source.s":   "",
			"foo/bar/shared_source.S":   "",
			"foo/bar/static_source.cpp": "",
			"foo/bar/static_source.cc":  "",
			"foo/bar/static_source.c":   "",
			"foo/bar/static_source.s":   "",
			"foo/bar/static_source.S":   "",
			"foo/bar/Android.bp": `
cc_library {
    name: "a",
    srcs: [
    "both_source.cpp",
    "both_source.cc",
    "both_source.c",
    "both_source.s",
    "both_source.S",
    ":both_filegroup",
  ],
    static: {
        srcs: [
          "static_source.cpp",
          "static_source.cc",
          "static_source.c",
          "static_source.s",
          "static_source.S",
          ":static_filegroup",
        ],
    },
    shared: {
        srcs: [
          "shared_source.cpp",
          "shared_source.cc",
          "shared_source.c",
          "shared_source.s",
          "shared_source.S",
          ":shared_filegroup",
        ],
    },
    bazel_module: { bp2build_available: true },
}

filegroup {
    name: "both_filegroup",
    srcs: [
        // Not relevant, handled by filegroup macro
  ],
}

filegroup {
    name: "shared_filegroup",
    srcs: [
        // Not relevant, handled by filegroup macro
  ],
}

filegroup {
    name: "static_filegroup",
    srcs: [
        // Not relevant, handled by filegroup macro
  ],
}
`,
		},
		Blueprint: soongCcLibraryPreamble,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
				"srcs": `[
        "both_source.cpp",
        "both_source.cc",
        ":both_filegroup_cpp_srcs",
        "static_source.cpp",
        "static_source.cc",
        ":static_filegroup_cpp_srcs",
    ]`,
				"srcs_as": `[
        "both_source.s",
        "both_source.S",
        ":both_filegroup_as_srcs",
        "static_source.s",
        "static_source.S",
        ":static_filegroup_as_srcs",
    ]`,
				"srcs_c": `[
        "both_source.c",
        ":both_filegroup_c_srcs",
        "static_source.c",
        ":static_filegroup_c_srcs",
    ]`,
			}),
			MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"local_includes": `["."]`,
				"srcs": `[
        "both_source.cpp",
        "both_source.cc",
        ":both_filegroup_cpp_srcs",
        "shared_source.cpp",
        "shared_source.cc",
        ":shared_filegroup_cpp_srcs",
    ]`,
				"srcs_as": `[
        "both_source.s",
        "both_source.S",
        ":both_filegroup_as_srcs",
        "shared_source.s",
        "shared_source.S",
        ":shared_filegroup_as_srcs",
    ]`,
				"srcs_c": `[
        "both_source.c",
        ":both_filegroup_c_srcs",
        "shared_source.c",
        ":shared_filegroup_c_srcs",
    ]`,
			})}})
}

func TestCcLibraryNonConfiguredVersionScriptAndDynamicList(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library non-configured version script and dynamic list",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "foo/bar",
		Filesystem: map[string]string{
			"foo/bar/Android.bp": `
cc_library {
    name: "a",
    srcs: ["a.cpp"],
    version_script: "v.map",
    dynamic_list: "dynamic.list",
    bazel_module: { bp2build_available: true },
    include_build_directory: false,
}
`,
		},
		Blueprint: soongCcLibraryPreamble,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"additional_linker_inputs": `[
        "v.map",
        "dynamic.list",
    ]`,
			"linkopts": `[
        "-Wl,--version-script,$(location v.map)",
        "-Wl,--dynamic-list,$(location dynamic.list)",
    ]`,
			"srcs":     `["a.cpp"]`,
			"features": `["android_cfi_exports_map"]`,
		}),
	},
	)
}

func TestCcLibraryConfiguredVersionScriptAndDynamicList(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library configured version script and dynamic list",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "foo/bar",
		Filesystem: map[string]string{
			"foo/bar/Android.bp": `
cc_library {
   name: "a",
   srcs: ["a.cpp"],
   arch: {
     arm: {
       version_script: "arm.map",
       dynamic_list: "dynamic_arm.list",
     },
     arm64: {
       version_script: "arm64.map",
       dynamic_list: "dynamic_arm64.list",
     },
   },

   bazel_module: { bp2build_available: true },
    include_build_directory: false,
}
    `,
		},
		Blueprint: soongCcLibraryPreamble,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"additional_linker_inputs": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [
            "arm.map",
            "dynamic_arm.list",
        ],
        "//build/bazel_common_rules/platforms/arch:arm64": [
            "arm64.map",
            "dynamic_arm64.list",
        ],
        "//conditions:default": [],
    })`,
			"linkopts": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [
            "-Wl,--version-script,$(location arm.map)",
            "-Wl,--dynamic-list,$(location dynamic_arm.list)",
        ],
        "//build/bazel_common_rules/platforms/arch:arm64": [
            "-Wl,--version-script,$(location arm64.map)",
            "-Wl,--dynamic-list,$(location dynamic_arm64.list)",
        ],
        "//conditions:default": [],
    })`,
			"srcs": `["a.cpp"]`,
			"features": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["android_cfi_exports_map"],
        "//build/bazel_common_rules/platforms/arch:arm64": ["android_cfi_exports_map"],
        "//conditions:default": [],
    })`,
		}),
	},
	)
}

func TestCcLibraryLdflagsSplitBySpaceExceptSoongAdded(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "ldflags are split by spaces except for the ones added by soong (version script and dynamic list)",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"version_script": "",
			"dynamic.list":   "",
		},
		Blueprint: `
cc_library {
    name: "foo",
    ldflags: [
        "--nospace_flag",
        "-z spaceflag",
    ],
    version_script: "version_script",
    dynamic_list: "dynamic.list",
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `["android_cfi_exports_map"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"additional_linker_inputs": `[
        "version_script",
        "dynamic.list",
    ]`,
				"features": `["android_cfi_exports_map"]`,
				"linkopts": `[
        "--nospace_flag",
        "-z",
        "spaceflag",
        "-Wl,--version-script,$(location version_script)",
        "-Wl,--dynamic-list,$(location dynamic.list)",
    ]`,
			}),
		},
	})
}

func TestCcLibrarySharedLibs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library shared_libs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"mylib"},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "mylib",
}

cc_library {
    name: "a",
    shared_libs: ["mylib",],
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"implementation_dynamic_deps": `[":mylib"]`,
		}),
	},
	)
}

func TestCcLibraryFeatures(t *testing.T) {
	expected_targets := []string{}
	expected_targets = append(expected_targets, makeCcLibraryTargets("a", AttrNameToString{
		"features": `[
        "disable_pack_relocations",
        "-no_undefined_symbols",
    ]`,
		"native_coverage": `False`,
		"srcs":            `["a.cpp"]`,
	})...)
	expected_targets = append(expected_targets, makeCcLibraryTargets("b", AttrNameToString{
		"features": `select({
        "//build/bazel_common_rules/platforms/arch:x86_64": [
            "disable_pack_relocations",
            "-no_undefined_symbols",
        ],
        "//conditions:default": [],
    })`,
		"native_coverage": `False`,
		"srcs":            `["b.cpp"]`,
	})...)
	expected_targets = append(expected_targets, makeCcLibraryTargets("c", AttrNameToString{
		"features": `select({
        "//build/bazel_common_rules/platforms/os:darwin": [
            "disable_pack_relocations",
            "-no_undefined_symbols",
        ],
        "//conditions:default": [],
    })`,
		"srcs": `["c.cpp"]`,
	})...)

	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library pack_relocations test",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "a",
    srcs: ["a.cpp"],
    pack_relocations: false,
    allow_undefined_symbols: true,
    include_build_directory: false,
    native_coverage: false,
}

cc_library {
    name: "b",
    srcs: ["b.cpp"],
    arch: {
        x86_64: {
            pack_relocations: false,
            allow_undefined_symbols: true,
        },
    },
    include_build_directory: false,
    native_coverage: false,
}

cc_library {
    name: "c",
    srcs: ["c.cpp"],
    target: {
        darwin: {
            pack_relocations: false,
            allow_undefined_symbols: true,
        },
    },
    include_build_directory: false,
}`,
		ExpectedBazelTargets: expected_targets,
	})
}

func TestCcLibrarySpacesInCopts(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library spaces in copts",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "a",
    cflags: ["-include header.h",],
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"copts": `[
        "-include",
        "header.h",
    ]`,
		}),
	},
	)
}

func TestCcLibraryCppFlagsGoesIntoCopts(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library cppflags usage",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `cc_library {
    name: "a",
    srcs: ["a.cpp"],
    cflags: ["-Wall"],
    cppflags: [
        "-fsigned-char",
        "-pedantic",
    ],
    arch: {
        arm64: {
            cppflags: ["-DARM64=1"],
        },
    },
    target: {
        android: {
            cppflags: ["-DANDROID=1"],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"copts": `["-Wall"]`,
			"cppflags": `[
        "-fsigned-char",
        "-pedantic",
    ] + select({
        "//build/bazel_common_rules/platforms/arch:arm64": ["-DARM64=1"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": ["-DANDROID=1"],
        "//conditions:default": [],
    })`,
			"srcs": `["a.cpp"]`,
		}),
	},
	)
}

func TestCcLibraryExcludeLibs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem:                 map[string]string{},
		StubbedBuildDefinitions: []string{"arm_whole_static_lib_excludes", "malloc_not_svelte_whole_static_lib",
			"arm_static_lib_excludes", "malloc_not_svelte_whole_static_lib_excludes", "arm_shared_lib_excludes",
			"malloc_not_svelte_static_lib_excludes", "arm_shared_lib_excludes", "malloc_not_svelte_shared_lib",
		},
		Blueprint: soongCcLibraryStaticPreamble + `
cc_library {
    name: "foo_static",
    srcs: ["common.c"],
    whole_static_libs: [
        "arm_whole_static_lib_excludes",
        "malloc_not_svelte_whole_static_lib_excludes"
    ],
    static_libs: [
        "arm_static_lib_excludes",
        "malloc_not_svelte_static_lib_excludes"
    ],
    shared_libs: [
        "arm_shared_lib_excludes",
    ],
    arch: {
        arm: {
            exclude_shared_libs: [
                 "arm_shared_lib_excludes",
            ],
            exclude_static_libs: [
                "arm_static_lib_excludes",
                "arm_whole_static_lib_excludes",
            ],
        },
    },
    product_variables: {
        malloc_not_svelte: {
            shared_libs: ["malloc_not_svelte_shared_lib"],
            whole_static_libs: ["malloc_not_svelte_whole_static_lib"],
            exclude_static_libs: [
                "malloc_not_svelte_static_lib_excludes",
                "malloc_not_svelte_whole_static_lib_excludes",
            ],
        },
    },
    include_build_directory: false,
}

cc_library {
    name: "arm_whole_static_lib_excludes",
}

cc_library {
    name: "malloc_not_svelte_whole_static_lib",
}

cc_library {
    name: "malloc_not_svelte_whole_static_lib_excludes",
}

cc_library {
    name: "arm_static_lib_excludes",
}

cc_library {
    name: "malloc_not_svelte_static_lib_excludes",
}

cc_library {
    name: "arm_shared_lib_excludes",
}

cc_library {
    name: "malloc_not_svelte_shared_lib",
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo_static", AttrNameToString{
			"implementation_deps": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [],
        "//conditions:default": [":arm_static_lib_excludes_bp2build_cc_library_static"],
    }) + select({
        "//build/bazel/product_config/config_settings:malloc_not_svelte": [],
        "//conditions:default": [":malloc_not_svelte_static_lib_excludes_bp2build_cc_library_static"],
    })`,
			"implementation_dynamic_deps": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [],
        "//conditions:default": [":arm_shared_lib_excludes"],
    }) + select({
        "//build/bazel/product_config/config_settings:malloc_not_svelte": [":malloc_not_svelte_shared_lib"],
        "//conditions:default": [],
    })`,
			"srcs_c": `["common.c"]`,
			"whole_archive_deps": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [],
        "//conditions:default": [":arm_whole_static_lib_excludes_bp2build_cc_library_static"],
    }) + select({
        "//build/bazel/product_config/config_settings:malloc_not_svelte": [":malloc_not_svelte_whole_static_lib_bp2build_cc_library_static"],
        "//conditions:default": [":malloc_not_svelte_whole_static_lib_excludes_bp2build_cc_library_static"],
    })`,
		}),
	},
	)
}

func TestCcLibraryProductVariablesHeaderLibs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem:                 map[string]string{},
		StubbedBuildDefinitions:    []string{"malloc_not_svelte_header_lib"},
		Blueprint: soongCcLibraryStaticPreamble + `
cc_library {
    name: "foo_static",
    srcs: ["common.c"],
    product_variables: {
        malloc_not_svelte: {
            header_libs: ["malloc_not_svelte_header_lib"],
        },
    },
    include_build_directory: false,
}

cc_library {
    name: "malloc_not_svelte_header_lib",
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo_static", AttrNameToString{
			"implementation_deps": `select({
        "//build/bazel/product_config/config_settings:malloc_not_svelte": [":malloc_not_svelte_header_lib"],
        "//conditions:default": [],
    })`,
			"srcs_c":                 `["common.c"]`,
			"target_compatible_with": `["//build/bazel_common_rules/platforms/os:android"]`,
		}),
	},
	)
}

func TestCCLibraryNoCrtTrue(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - nocrt: true disables feature",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    nocrt: true,
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"features": `["-link_crt"]`,
			"srcs":     `["impl.cpp"]`,
		}),
	},
	)
}

func TestCCLibraryNoCrtFalse(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - nocrt: false - does not emit attribute",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    nocrt: false,
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"srcs": `["impl.cpp"]`,
		}),
	})
}

func TestCCLibraryNoCrtArchVariant(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - nocrt in select",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    arch: {
        arm: {
            nocrt: true,
        },
        x86: {
            nocrt: false,
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"features": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["-link_crt"],
        "//conditions:default": [],
    })`,
			"srcs": `["impl.cpp"]`,
		}),
	})
}

func TestCCLibraryNoLibCrtTrue(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    no_libcrt: true,
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"features": `["-use_libcrt"]`,
			"srcs":     `["impl.cpp"]`,
		}),
	})
}

func makeCcLibraryTargets(name string, attrs AttrNameToString) []string {
	STATIC_ONLY_ATTRS := map[string]bool{}
	SHARED_ONLY_ATTRS := map[string]bool{
		"link_crt":                 true,
		"additional_linker_inputs": true,
		"linkopts":                 true,
		"strip":                    true,
		"inject_bssl_hash":         true,
		"stubs_symbol_file":        true,
		"use_version_lib":          true,
	}

	sharedAttrs := AttrNameToString{}
	staticAttrs := AttrNameToString{}
	for key, val := range attrs {
		if _, staticOnly := STATIC_ONLY_ATTRS[key]; !staticOnly {
			sharedAttrs[key] = val
		}
		if _, sharedOnly := SHARED_ONLY_ATTRS[key]; !sharedOnly {
			staticAttrs[key] = val
		}
	}
	sharedTarget := MakeBazelTarget("cc_library_shared", name, sharedAttrs)
	staticTarget := MakeBazelTarget("cc_library_static", name+"_bp2build_cc_library_static", staticAttrs)

	return []string{staticTarget, sharedTarget}
}

func TestCCLibraryNoLibCrtFalse(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    no_libcrt: false,
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"srcs": `["impl.cpp"]`,
		}),
	})
}

func TestCCLibraryNoLibCrtArchVariant(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    arch: {
        arm: {
            no_libcrt: true,
        },
        x86: {
            no_libcrt: true,
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"srcs": `["impl.cpp"]`,
			"features": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["-use_libcrt"],
        "//build/bazel_common_rules/platforms/arch:x86": ["-use_libcrt"],
        "//conditions:default": [],
    })`,
		}),
	})
}

func TestCCLibraryNoLibCrtArchAndTargetVariant(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    arch: {
        arm: {
            no_libcrt: true,
        },
        x86: {
            no_libcrt: true,
        },
    },
    target: {
        darwin: {
            no_libcrt: true,
        }
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"features": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["-use_libcrt"],
        "//build/bazel_common_rules/platforms/arch:x86": ["-use_libcrt"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:darwin": ["-use_libcrt"],
        "//conditions:default": [],
    })`,
			"srcs": `["impl.cpp"]`,
		}),
	})
}

func TestCCLibraryNoLibCrtArchAndTargetVariantConflict(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"impl.cpp": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["impl.cpp"],
    arch: {
        arm: {
            no_libcrt: true,
        },
        // This is expected to override the value for darwin_x86_64.
        x86_64: {
            no_libcrt: true,
        },
    },
    target: {
        darwin: {
            no_libcrt: false,
        }
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"srcs": `["impl.cpp"]`,
			"features": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["-use_libcrt"],
        "//build/bazel_common_rules/platforms/arch:x86_64": ["-use_libcrt"],
        "//conditions:default": [],
    })`,
		}),
	})
}

func TestCcLibraryStrip(t *testing.T) {
	expectedTargets := []string{}
	expectedTargets = append(expectedTargets, makeCcLibraryTargets("all", AttrNameToString{
		"strip": `{
        "all": True,
    }`,
	})...)
	expectedTargets = append(expectedTargets, makeCcLibraryTargets("keep_symbols", AttrNameToString{
		"strip": `{
        "keep_symbols": True,
    }`,
	})...)
	expectedTargets = append(expectedTargets, makeCcLibraryTargets("keep_symbols_and_debug_frame", AttrNameToString{
		"strip": `{
        "keep_symbols_and_debug_frame": True,
    }`,
	})...)
	expectedTargets = append(expectedTargets, makeCcLibraryTargets("keep_symbols_list", AttrNameToString{
		"strip": `{
        "keep_symbols_list": ["symbol"],
    }`,
	})...)
	expectedTargets = append(expectedTargets, makeCcLibraryTargets("none", AttrNameToString{
		"strip": `{
        "none": True,
    }`,
	})...)
	expectedTargets = append(expectedTargets, makeCcLibraryTargets("nothing", AttrNameToString{})...)

	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library strip args",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "nothing",
    include_build_directory: false,
}
cc_library {
    name: "keep_symbols",
    strip: {
        keep_symbols: true,
    },
    include_build_directory: false,
}
cc_library {
    name: "keep_symbols_and_debug_frame",
    strip: {
        keep_symbols_and_debug_frame: true,
    },
    include_build_directory: false,
}
cc_library {
    name: "none",
    strip: {
        none: true,
    },
    include_build_directory: false,
}
cc_library {
    name: "keep_symbols_list",
    strip: {
        keep_symbols_list: ["symbol"],
    },
    include_build_directory: false,
}
cc_library {
    name: "all",
    strip: {
        all: true,
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: expectedTargets,
	})
}

func TestCcLibraryStripWithArch(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library strip args",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "multi-arch",
    target: {
        darwin: {
            strip: {
                keep_symbols_list: ["foo", "bar"]
            }
        },
    },
    arch: {
        arm: {
            strip: {
                keep_symbols_and_debug_frame: true,
            },
        },
        arm64: {
            strip: {
                keep_symbols: true,
            },
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("multi-arch", AttrNameToString{
			"strip": `{
        "keep_symbols": select({
            "//build/bazel_common_rules/platforms/arch:arm64": True,
            "//conditions:default": None,
        }),
        "keep_symbols_and_debug_frame": select({
            "//build/bazel_common_rules/platforms/arch:arm": True,
            "//conditions:default": None,
        }),
        "keep_symbols_list": select({
            "//build/bazel_common_rules/platforms/os:darwin": [
                "foo",
                "bar",
            ],
            "//conditions:default": [],
        }),
    }`,
		}),
	},
	)
}

func TestCcLibrary_SystemSharedLibsRootEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs empty at root",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "root_empty",
    system_shared_libs: [],
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("root_empty", AttrNameToString{
			"system_dynamic_deps": `[]`,
		}),
	},
	)
}

func TestCcLibrary_SystemSharedLibsStaticEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs empty for static variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "static_empty",
    static: {
        system_shared_libs: [],
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "static_empty_bp2build_cc_library_static", AttrNameToString{
				"system_dynamic_deps": "[]",
			}),
			MakeBazelTarget("cc_library_shared", "static_empty", AttrNameToString{}),
		},
	})
}

func TestCcLibrary_SystemSharedLibsSharedEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs empty for shared variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "shared_empty",
    shared: {
        system_shared_libs: [],
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "shared_empty_bp2build_cc_library_static", AttrNameToString{}),
			MakeBazelTarget("cc_library_shared", "shared_empty", AttrNameToString{
				"system_dynamic_deps": "[]",
			}),
		},
	})
}

func TestCcLibrary_SystemSharedLibsSharedBionicEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs empty for shared, bionic variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "shared_empty",
    target: {
        bionic: {
            shared: {
                system_shared_libs: [],
            }
        }
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "shared_empty_bp2build_cc_library_static", AttrNameToString{}),
			MakeBazelTarget("cc_library_shared", "shared_empty", AttrNameToString{
				"system_dynamic_deps": "[]",
			}),
		},
	})
}

func TestCcLibrary_SystemSharedLibsLinuxBionicEmpty(t *testing.T) {
	// Note that this behavior is technically incorrect (it's a simplification).
	// The correct behavior would be if bp2build wrote `system_dynamic_deps = []`
	// only for linux_bionic, but `android` had `["libc", "libdl", "libm"].
	// b/195791252 tracks the fix.
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs empty for linux_bionic variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"libc_musl"},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
	name: "libc_musl",
}

cc_library {
    name: "target_linux_bionic_empty",
    target: {
        linux_bionic: {
            system_shared_libs: [],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("target_linux_bionic_empty", AttrNameToString{
			"system_dynamic_deps": `select({
        "//build/bazel_common_rules/platforms/os:linux_musl": [":libc_musl"],
        "//conditions:default": [],
    })`,
		}),
	},
	)
}

func TestCcLibrary_SystemSharedLibsBionicEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs empty for bionic variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"libc_musl"},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
	name: "libc_musl",
}

cc_library {
    name: "target_bionic_empty",
    target: {
        bionic: {
            system_shared_libs: [],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("target_bionic_empty", AttrNameToString{
			"system_dynamic_deps": `select({
        "//build/bazel_common_rules/platforms/os:linux_musl": [":libc_musl"],
        "//conditions:default": [],
    })`,
		}),
	},
	)
}

func TestCcLibrary_SystemSharedLibsMuslEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_lib empty for musl variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"libc_musl"},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
		name: "libc_musl",
}

cc_library {
    name: "target_musl_empty",
    target: {
        musl: {
            system_shared_libs: [],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("target_musl_empty", AttrNameToString{
			"system_dynamic_deps": `[]`,
		}),
	})
}

func TestCcLibrary_SystemSharedLibsLinuxMuslEmpty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_lib empty for linux_musl variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"libc_musl"},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
		name: "libc_musl",
}

cc_library {
    name: "target_linux_musl_empty",
    target: {
        linux_musl: {
            system_shared_libs: [],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("target_linux_musl_empty", AttrNameToString{
			"system_dynamic_deps": `[]`,
		}),
	})
}
func TestCcLibrary_SystemSharedLibsSharedAndRoot(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library system_shared_libs set for shared and root",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"libc", "libm"},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "libc",
}
cc_library {
    name: "libm",
}

cc_library {
    name: "foo",
    system_shared_libs: ["libc"],
    shared: {
        system_shared_libs: ["libm"],
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"system_dynamic_deps": `[":libc"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"system_dynamic_deps": `[
        ":libc",
        ":libm",
    ]`,
			}),
		},
	})
}

func TestCcLibraryOsSelects(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - selects for all os targets",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem:                 map[string]string{},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo-lib",
    srcs: ["base.cpp"],
    target: {
        android: {
            srcs: ["android.cpp"],
        },
        linux: {
            srcs: ["linux.cpp"],
        },
        linux_glibc: {
            srcs: ["linux_glibc.cpp"],
        },
        darwin: {
            srcs: ["darwin.cpp"],
        },
        bionic: {
            srcs: ["bionic.cpp"],
        },
        linux_musl: {
            srcs: ["linux_musl.cpp"],
        },
        windows: {
            srcs: ["windows.cpp"],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo-lib", AttrNameToString{
			"srcs": `["base.cpp"] + select({
        "//build/bazel_common_rules/platforms/os:android": [
            "linux.cpp",
            "bionic.cpp",
            "android.cpp",
        ],
        "//build/bazel_common_rules/platforms/os:darwin": ["darwin.cpp"],
        "//build/bazel_common_rules/platforms/os:linux_bionic": [
            "linux.cpp",
            "bionic.cpp",
        ],
        "//build/bazel_common_rules/platforms/os:linux_glibc": [
            "linux.cpp",
            "linux_glibc.cpp",
        ],
        "//build/bazel_common_rules/platforms/os:linux_musl": [
            "linux.cpp",
            "linux_musl.cpp",
        ],
        "//build/bazel_common_rules/platforms/os:windows": ["windows.cpp"],
        "//conditions:default": [],
    })`,
		}),
	},
	)
}

func TestLibcryptoHashInjection(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library - libcrypto hash injection",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem:                 map[string]string{},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "libcrypto",
    target: {
        android: {
            inject_bssl_hash: true,
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("libcrypto", AttrNameToString{
			"inject_bssl_hash": `select({
        "//build/bazel_common_rules/platforms/os:android": True,
        "//conditions:default": None,
    })`,
		}),
	},
	)
}

func TestCcLibraryCppStdWithGnuExtensions_ConvertsToFeatureAttr(t *testing.T) {
	type testCase struct {
		cpp_std        string
		c_std          string
		gnu_extensions string
		bazel_cpp_std  string
		bazel_c_std    string
	}

	testCases := []testCase{
		// Existing usages of cpp_std in AOSP are:
		// experimental, c++11, c++17, c++2a, c++98, gnu++11, gnu++17
		//
		// not set, only emit if gnu_extensions is disabled. the default (gnu+17
		// is set in the toolchain.)
		{cpp_std: "", gnu_extensions: "", bazel_cpp_std: ""},
		{cpp_std: "", gnu_extensions: "false", bazel_cpp_std: "cpp_std_default_no_gnu", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "", gnu_extensions: "true", bazel_cpp_std: ""},
		// experimental defaults to gnu++2a
		{cpp_std: "experimental", gnu_extensions: "", bazel_cpp_std: "cpp_std_experimental"},
		{cpp_std: "experimental", gnu_extensions: "false", bazel_cpp_std: "cpp_std_experimental_no_gnu", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "experimental", gnu_extensions: "true", bazel_cpp_std: "cpp_std_experimental"},
		// Explicitly setting a c++ std does not use replace gnu++ std even if
		// gnu_extensions is true.
		// "c++11",
		{cpp_std: "c++11", gnu_extensions: "", bazel_cpp_std: "c++11"},
		{cpp_std: "c++11", gnu_extensions: "false", bazel_cpp_std: "c++11", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "c++11", gnu_extensions: "true", bazel_cpp_std: "c++11"},
		// "c++17",
		{cpp_std: "c++17", gnu_extensions: "", bazel_cpp_std: "c++17"},
		{cpp_std: "c++17", gnu_extensions: "false", bazel_cpp_std: "c++17", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "c++17", gnu_extensions: "true", bazel_cpp_std: "c++17"},
		// "c++2a",
		{cpp_std: "c++2a", gnu_extensions: "", bazel_cpp_std: "c++2a"},
		{cpp_std: "c++2a", gnu_extensions: "false", bazel_cpp_std: "c++2a", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "c++2a", gnu_extensions: "true", bazel_cpp_std: "c++2a"},
		// "c++98",
		{cpp_std: "c++98", gnu_extensions: "", bazel_cpp_std: "c++98"},
		{cpp_std: "c++98", gnu_extensions: "false", bazel_cpp_std: "c++98", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "c++98", gnu_extensions: "true", bazel_cpp_std: "c++98"},
		// gnu++ is replaced with c++ if gnu_extensions is explicitly false.
		// "gnu++11",
		{cpp_std: "gnu++11", gnu_extensions: "", bazel_cpp_std: "gnu++11"},
		{cpp_std: "gnu++11", gnu_extensions: "false", bazel_cpp_std: "c++11", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "gnu++11", gnu_extensions: "true", bazel_cpp_std: "gnu++11"},
		// "gnu++17",
		{cpp_std: "gnu++17", gnu_extensions: "", bazel_cpp_std: "gnu++17"},
		{cpp_std: "gnu++17", gnu_extensions: "false", bazel_cpp_std: "c++17", bazel_c_std: "c_std_default_no_gnu"},
		{cpp_std: "gnu++17", gnu_extensions: "true", bazel_cpp_std: "gnu++17"},

		// some c_std test cases
		{c_std: "experimental", gnu_extensions: "", bazel_c_std: "c_std_experimental"},
		{c_std: "experimental", gnu_extensions: "false", bazel_cpp_std: "cpp_std_default_no_gnu", bazel_c_std: "c_std_experimental_no_gnu"},
		{c_std: "experimental", gnu_extensions: "true", bazel_c_std: "c_std_experimental"},
		{c_std: "gnu11", cpp_std: "gnu++17", gnu_extensions: "", bazel_cpp_std: "gnu++17", bazel_c_std: "gnu11"},
		{c_std: "gnu11", cpp_std: "gnu++17", gnu_extensions: "false", bazel_cpp_std: "c++17", bazel_c_std: "c11"},
		{c_std: "gnu11", cpp_std: "gnu++17", gnu_extensions: "true", bazel_cpp_std: "gnu++17", bazel_c_std: "gnu11"},
	}
	for i, tc := range testCases {
		name := fmt.Sprintf("cpp std: %q, c std: %q, gnu_extensions: %q", tc.cpp_std, tc.c_std, tc.gnu_extensions)
		t.Run(name, func(t *testing.T) {
			name_prefix := fmt.Sprintf("a_%v", i)
			cppStdProp := ""
			if tc.cpp_std != "" {
				cppStdProp = fmt.Sprintf("    cpp_std: \"%s\",", tc.cpp_std)
			}
			cStdProp := ""
			if tc.c_std != "" {
				cStdProp = fmt.Sprintf("    c_std: \"%s\",", tc.c_std)
			}
			gnuExtensionsProp := ""
			if tc.gnu_extensions != "" {
				gnuExtensionsProp = fmt.Sprintf("    gnu_extensions: %s,", tc.gnu_extensions)
			}
			attrs := AttrNameToString{}
			if tc.bazel_cpp_std != "" {
				attrs["cpp_std"] = fmt.Sprintf(`"%s"`, tc.bazel_cpp_std)
			}
			if tc.bazel_c_std != "" {
				attrs["c_std"] = fmt.Sprintf(`"%s"`, tc.bazel_c_std)
			}

			runCcLibraryTestCase(t, Bp2buildTestCase{
				Description: fmt.Sprintf(
					"cc_library with cpp_std: %s and gnu_extensions: %s", tc.cpp_std, tc.gnu_extensions),
				ModuleTypeUnderTest:        "cc_library",
				ModuleTypeUnderTestFactory: cc.LibraryFactory,
				Blueprint: soongCcLibraryPreamble + fmt.Sprintf(`
cc_library {
	name: "%s_full",
%s // cpp_std: *string
%s // c_std: *string
%s // gnu_extensions: *bool
	include_build_directory: false,
}
`, name_prefix, cppStdProp, cStdProp, gnuExtensionsProp),
				ExpectedBazelTargets: makeCcLibraryTargets(name_prefix+"_full", attrs),
			})

			runCcLibraryStaticTestCase(t, Bp2buildTestCase{
				Description: fmt.Sprintf(
					"cc_library_static with cpp_std: %s and gnu_extensions: %s", tc.cpp_std, tc.gnu_extensions),
				ModuleTypeUnderTest:        "cc_library_static",
				ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
				Blueprint: soongCcLibraryPreamble + fmt.Sprintf(`
cc_library_static {
	name: "%s_static",
%s // cpp_std: *string
%s // c_std: *string
%s // gnu_extensions: *bool
	include_build_directory: false,
}
`, name_prefix, cppStdProp, cStdProp, gnuExtensionsProp),
				ExpectedBazelTargets: []string{
					MakeBazelTarget("cc_library_static", name_prefix+"_static", attrs),
				},
			})

			runCcLibrarySharedTestCase(t, Bp2buildTestCase{
				Description: fmt.Sprintf(
					"cc_library_shared with cpp_std: %s and gnu_extensions: %s", tc.cpp_std, tc.gnu_extensions),
				ModuleTypeUnderTest:        "cc_library_shared",
				ModuleTypeUnderTestFactory: cc.LibrarySharedFactory,
				Blueprint: soongCcLibraryPreamble + fmt.Sprintf(`
cc_library_shared {
	name: "%s_shared",
%s // cpp_std: *string
%s // c_std: *string
%s // gnu_extensions: *bool
	include_build_directory: false,
}
`, name_prefix, cppStdProp, cStdProp, gnuExtensionsProp),
				ExpectedBazelTargets: []string{
					MakeBazelTarget("cc_library_shared", name_prefix+"_shared", attrs),
				},
			})
		})
	}
}

func TestCcLibraryProtoSimple(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs": `["foo.proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
				"deps":                              `[":libprotobuf-cpp-lite"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":                      `[":libprotobuf-cpp-lite"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoNoCanonicalPathFromRoot(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: { canonical_path_from_root: false},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs":                `["foo.proto"]`,
				"strip_import_prefix": `""`,
			}), MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
				"deps":                              `[":libprotobuf-cpp-lite"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":                      `[":libprotobuf-cpp-lite"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoExplicitCanonicalPathFromRoot(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: { canonical_path_from_root: true},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs": `["foo.proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
				"deps":                              `[":libprotobuf-cpp-lite"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":                      `[":libprotobuf-cpp-lite"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoFull(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: {
		type: "full",
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs": `["foo.proto"]`,
			}), MakeBazelTarget("cc_proto_library", "foo_cc_proto", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_proto"]`,
				"deps":                              `[":libprotobuf-cpp-full"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":                      `[":libprotobuf-cpp-full"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto"]`,
			}),
		},
	})
}

func TestCcLibraryProtoLite(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: {
		type: "lite",
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs": `["foo.proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
				"deps":                              `[":libprotobuf-cpp-lite"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":                      `[":libprotobuf-cpp-lite"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoExportHeaders(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs": `["foo.proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoIncludeDirs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: {
		include_dirs: ["external/protobuf/src"],
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
				"srcs": `["foo.proto"]`,
				"deps": `["//external/protobuf:libprotobuf-proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
				"deps": `[":foo_proto"]`,
			}), MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"deps":                              `[":libprotobuf-cpp-lite"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"dynamic_deps":                      `[":libprotobuf-cpp-lite"]`,
				"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoIncludeDirsUnknown(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.proto"],
	proto: {
		include_dirs: ["external/protobuf/abc"],
	},
	include_build_directory: false,
}`,
		ExpectedErr: fmt.Errorf("module \"foo\": TODO: Add support for proto.include_dir: external/protobuf/abc. This directory does not contain an Android.bp file"),
	})
}

func TestCcLibraryConvertedProtoFilegroups(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `
filegroup {
	name: "a_fg_proto",
	srcs: ["a_fg.proto"],
}

cc_library {
	name: "a",
	srcs: [
    ":a_fg_proto",
    "a.proto",
  ],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "a_proto", AttrNameToString{
				"deps": `[":a_fg_proto_bp2build_converted"]`,
				"srcs": `["a.proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "a_cc_proto_lite", AttrNameToString{
				"deps": `[
        ":a_fg_proto_bp2build_converted",
        ":a_proto",
    ]`,
			}), MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
			}), MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
			}), MakeBazelTargetNoRestrictions("proto_library", "a_fg_proto_proto", AttrNameToString{
				"srcs": `["a_fg.proto"]`,
				"tags": `[
        "apex_available=//apex_available:anyapex",
        "manual",
    ]`,
			}), MakeBazelTargetNoRestrictions("alias", "a_fg_proto_bp2build_converted", AttrNameToString{
				"actual": `"//.:a_fg_proto_proto"`,
				"tags": `[
        "apex_available=//apex_available:anyapex",
        "manual",
    ]`,
			}), MakeBazelTargetNoRestrictions("filegroup", "a_fg_proto", AttrNameToString{
				"srcs": `["a_fg.proto"]`,
			}),
		},
	})
}

func TestCcLibraryConvertedProtoFilegroupsNoProtoFiles(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `
filegroup {
	name: "a_fg_proto",
	srcs: ["a_fg.proto"],
}

cc_library {
	name: "a",
	srcs: [
    ":a_fg_proto",
  ],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_lite_proto_library", "a_cc_proto_lite", AttrNameToString{
				"deps": `[":a_fg_proto_bp2build_converted"]`,
			}), MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
			}), MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
			}), MakeBazelTargetNoRestrictions("proto_library", "a_fg_proto_proto", AttrNameToString{
				"srcs": `["a_fg.proto"]`,
				"tags": `[
        "apex_available=//apex_available:anyapex",
        "manual",
    ]`,
			}), MakeBazelTargetNoRestrictions("alias", "a_fg_proto_bp2build_converted", AttrNameToString{
				"actual": `"//.:a_fg_proto_proto"`,
				"tags": `[
        "apex_available=//apex_available:anyapex",
        "manual",
    ]`,
			}), MakeBazelTargetNoRestrictions("filegroup", "a_fg_proto", AttrNameToString{
				"srcs": `["a_fg.proto"]`,
			}),
		},
	})
}

func TestCcLibraryExternalConvertedProtoFilegroups(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"//path/to/A:a_fg_proto"},
		Filesystem: map[string]string{
			"path/to/A/Android.bp": `
filegroup {
	name: "a_fg_proto",
	srcs: ["a_fg.proto"],
}`,
		},
		Blueprint: soongCcProtoPreamble + `
cc_library {
	name: "a",
	srcs: [
    ":a_fg_proto",
    "a.proto",
  ],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "a_proto", AttrNameToString{
				"deps": `["//path/to/A:a_fg_proto_bp2build_converted"]`,
				"srcs": `["a.proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "a_cc_proto_lite", AttrNameToString{
				"deps": `[
        "//path/to/A:a_fg_proto_bp2build_converted",
        ":a_proto",
    ]`,
			}), MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
			}), MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
			}),
		},
	})
}

func TestCcLibraryProtoFilegroups(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"a_fg_proto", "b_protos", "c-proto-srcs", "proto-srcs-d"},
		Blueprint: soongCcProtoPreamble +
			simpleModule("filegroup", "a_fg_proto") +
			simpleModule("filegroup", "b_protos") +
			simpleModule("filegroup", "c-proto-srcs") +
			simpleModule("filegroup", "proto-srcs-d") + `
cc_library {
	name: "a",
	srcs: [":a_fg_proto"],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}

cc_library {
	name: "b",
	srcs: [":b_protos"],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}

cc_library {
	name: "c",
	srcs: [":c-proto-srcs"],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}

cc_library {
	name: "d",
	srcs: [":proto-srcs-d"],
	proto: {
		export_proto_headers: true,
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("proto_library", "a_proto", AttrNameToString{
				"srcs": `[":a_fg_proto"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "a_cc_proto_lite", AttrNameToString{
				"deps": `[":a_proto"]`,
			}), MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
				"srcs":               `[":a_fg_proto_cpp_srcs"]`,
				"srcs_as":            `[":a_fg_proto_as_srcs"]`,
				"srcs_c":             `[":a_fg_proto_c_srcs"]`,
			}), MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":a_cc_proto_lite"]`,
				"srcs":               `[":a_fg_proto_cpp_srcs"]`,
				"srcs_as":            `[":a_fg_proto_as_srcs"]`,
				"srcs_c":             `[":a_fg_proto_c_srcs"]`,
			}), MakeBazelTarget("proto_library", "b_proto", AttrNameToString{
				"srcs": `[":b_protos"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "b_cc_proto_lite", AttrNameToString{
				"deps": `[":b_proto"]`,
			}), MakeBazelTarget("cc_library_static", "b_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":b_cc_proto_lite"]`,
				"srcs":               `[":b_protos_cpp_srcs"]`,
				"srcs_as":            `[":b_protos_as_srcs"]`,
				"srcs_c":             `[":b_protos_c_srcs"]`,
			}), MakeBazelTarget("cc_library_shared", "b", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":b_cc_proto_lite"]`,
				"srcs":               `[":b_protos_cpp_srcs"]`,
				"srcs_as":            `[":b_protos_as_srcs"]`,
				"srcs_c":             `[":b_protos_c_srcs"]`,
			}), MakeBazelTarget("proto_library", "c_proto", AttrNameToString{
				"srcs": `[":c-proto-srcs"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "c_cc_proto_lite", AttrNameToString{
				"deps": `[":c_proto"]`,
			}), MakeBazelTarget("cc_library_static", "c_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":c_cc_proto_lite"]`,
				"srcs":               `[":c-proto-srcs_cpp_srcs"]`,
				"srcs_as":            `[":c-proto-srcs_as_srcs"]`,
				"srcs_c":             `[":c-proto-srcs_c_srcs"]`,
			}), MakeBazelTarget("cc_library_shared", "c", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":c_cc_proto_lite"]`,
				"srcs":               `[":c-proto-srcs_cpp_srcs"]`,
				"srcs_as":            `[":c-proto-srcs_as_srcs"]`,
				"srcs_c":             `[":c-proto-srcs_c_srcs"]`,
			}), MakeBazelTarget("proto_library", "d_proto", AttrNameToString{
				"srcs": `[":proto-srcs-d"]`,
			}), MakeBazelTarget("cc_lite_proto_library", "d_cc_proto_lite", AttrNameToString{
				"deps": `[":d_proto"]`,
			}), MakeBazelTarget("cc_library_static", "d_bp2build_cc_library_static", AttrNameToString{
				"deps":               `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":d_cc_proto_lite"]`,
				"srcs":               `[":proto-srcs-d_cpp_srcs"]`,
				"srcs_as":            `[":proto-srcs-d_as_srcs"]`,
				"srcs_c":             `[":proto-srcs-d_c_srcs"]`,
			}), MakeBazelTarget("cc_library_shared", "d", AttrNameToString{
				"dynamic_deps":       `[":libprotobuf-cpp-lite"]`,
				"whole_archive_deps": `[":d_cc_proto_lite"]`,
				"srcs":               `[":proto-srcs-d_cpp_srcs"]`,
				"srcs_as":            `[":proto-srcs-d_as_srcs"]`,
				"srcs_c":             `[":proto-srcs-d_c_srcs"]`,
			}),
		},
	})
}

func TestCcLibraryDisabledArchAndTarget(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.cpp"],
	host_supported: true,
	target: {
		darwin: {
			enabled: false,
		},
		windows: {
			enabled: false,
		},
		linux_glibc_x86: {
			enabled: false,
		},
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo", AttrNameToString{
			"srcs": `["foo.cpp"]`,
			"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os_arch:darwin_arm64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:darwin_x86_64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:linux_glibc_x86": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:windows_x86": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:windows_x86_64": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
		}),
	})
}

func TestCcLibraryDisabledArchAndTargetWithDefault(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.cpp"],
  enabled: false,
	host_supported: true,
	target: {
		darwin: {
			enabled: true,
		},
		windows: {
			enabled: false,
		},
		linux_glibc_x86: {
			enabled: false,
		},
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo", AttrNameToString{
			"srcs": `["foo.cpp"]`,
			"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os_arch:darwin_arm64": [],
        "//build/bazel_common_rules/platforms/os_arch:darwin_x86_64": [],
        "//conditions:default": ["@platforms//:incompatible"],
    })`,
		}),
	})
}

func TestCcLibrarySharedDisabled(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	srcs: ["foo.cpp"],
	enabled: false,
	shared: {
		enabled: true,
	},
	target: {
		android: {
			shared: {
				enabled: false,
			},
		}
  },
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
			"srcs":                   `["foo.cpp"]`,
			"target_compatible_with": `["@platforms//:incompatible"]`,
		}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
			"srcs": `["foo.cpp"]`,
			"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
		}),
		},
	})
}

func TestCcLibraryStaticDisabledForSomeArch(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	host_supported: true,
	srcs: ["foo.cpp"],
	shared: {
		enabled: false
	},
	target: {
		darwin: {
			enabled: true,
		},
		windows: {
			enabled: false,
		},
		linux_glibc_x86: {
			shared: {
				enabled: true,
			},
		},
	},
	include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
			"srcs": `["foo.cpp"]`,
			"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:windows": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
		}), MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
			"srcs": `["foo.cpp"]`,
			"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os_arch:darwin_arm64": [],
        "//build/bazel_common_rules/platforms/os_arch:darwin_x86_64": [],
        "//build/bazel_common_rules/platforms/os_arch:linux_glibc_x86": [],
        "//conditions:default": ["@platforms//:incompatible"],
    })`,
		}),
		}})
}

func TestCcLibraryStubs(t *testing.T) {
	expectedBazelTargets := makeCcLibraryTargets("a", AttrNameToString{
		"stubs_symbol_file": `"a.map.txt"`,
	})
	expectedBazelTargets = append(expectedBazelTargets, makeCcStubSuiteTargets("a", AttrNameToString{
		"api_surface":          `"module-libapi"`,
		"soname":               `"a.so"`,
		"source_library_label": `"//foo/bar:a"`,
		"stubs_symbol_file":    `"a.map.txt"`,
		"stubs_versions": `[
        "28",
        "29",
        "current",
    ]`,
	}))
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library stubs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "foo/bar",
		Filesystem: map[string]string{
			"foo/bar/Android.bp": `
cc_library {
    name: "a",
    stubs: { symbol_file: "a.map.txt", versions: ["28", "29", "current"] },
    bazel_module: { bp2build_available: true },
    include_build_directory: false,
}
`,
		},
		Blueprint:            soongCcLibraryPreamble,
		ExpectedBazelTargets: expectedBazelTargets,
	},
	)
}

func TestCcLibraryStubsAcrossConfigsDuplicatesRemoved(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "stub target generation of the same lib across configs should not result in duplicates",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"bar.map.txt": "",
		},
		StubbedBuildDefinitions: []string{"barlib"},
		Blueprint: `
cc_library {
	name: "barlib",
	stubs: { symbol_file: "bar.map.txt", versions: ["28", "29", "current"] },
}
cc_library {
	name: "foolib",
	shared_libs: ["barlib"],
	target: {
		android: {
			shared_libs: ["barlib"],
		},
	},
	bazel_module: { bp2build_available: true },
	apex_available: ["foo"],
}`,
		ExpectedBazelTargets: makeCcLibraryTargets("foolib", AttrNameToString{
			"implementation_dynamic_deps": `select({
        "//build/bazel/rules/apex:foo": ["@api_surfaces//module-libapi/current:barlib"],
        "//conditions:default": [":barlib"],
    })`,
			"local_includes": `["."]`,
			"tags":           `["apex_available=foo"]`,
		}),
	})
}

func TestCcLibraryExcludesLibsHost(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"bar.map.txt": "",
		},
		StubbedBuildDefinitions: []string{"bazlib", "quxlib", "barlib"},
		Blueprint: simpleModule("cc_library", "bazlib") + `
cc_library {
	name: "quxlib",
	stubs: { symbol_file: "bar.map.txt", versions: ["current"] },
}
cc_library {
	name: "barlib",
	stubs: { symbol_file: "bar.map.txt", versions: ["28", "29", "current"] },
}
cc_library {
	name: "foolib",
	shared_libs: ["barlib", "quxlib"],
	target: {
		host: {
			shared_libs: ["bazlib"],
			exclude_shared_libs: ["barlib"],
		},
	},
	include_build_directory: false,
	bazel_module: { bp2build_available: true },
	apex_available: ["foo"],
}`,
		ExpectedBazelTargets: makeCcLibraryTargets("foolib", AttrNameToString{
			"implementation_dynamic_deps": `select({
        "//build/bazel_common_rules/platforms/os:darwin": [":bazlib"],
        "//build/bazel_common_rules/platforms/os:linux_bionic": [":bazlib"],
        "//build/bazel_common_rules/platforms/os:linux_glibc": [":bazlib"],
        "//build/bazel_common_rules/platforms/os:linux_musl": [":bazlib"],
        "//build/bazel_common_rules/platforms/os:windows": [":bazlib"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel/rules/apex:foo": [
            "@api_surfaces//module-libapi/current:barlib",
            "@api_surfaces//module-libapi/current:quxlib",
        ],
        "//build/bazel_common_rules/platforms/os:darwin": [":quxlib"],
        "//build/bazel_common_rules/platforms/os:linux_bionic": [":quxlib"],
        "//build/bazel_common_rules/platforms/os:linux_glibc": [":quxlib"],
        "//build/bazel_common_rules/platforms/os:linux_musl": [":quxlib"],
        "//build/bazel_common_rules/platforms/os:windows": [":quxlib"],
        "//conditions:default": [
            ":barlib",
            ":quxlib",
        ],
    })`,
			"tags": `["apex_available=foo"]`,
		}),
	})
}

func TestCcLibraryEscapeLdflags(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcProtoPreamble + `cc_library {
	name: "foo",
	ldflags: ["-Wl,--rpath,${ORIGIN}"],
	include_build_directory: false,
}`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo", AttrNameToString{
			"linkopts": `["-Wl,--rpath,$${ORIGIN}"]`,
		}),
	})
}

func TestCcLibraryConvertLex(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"foo.c":   "",
			"bar.cc":  "",
			"foo1.l":  "",
			"bar1.ll": "",
			"foo2.l":  "",
			"bar2.ll": "",
		},
		Blueprint: `cc_library {
	name: "foo_lib",
	srcs: ["foo.c", "bar.cc", "foo1.l", "foo2.l", "bar1.ll", "bar2.ll"],
	lex: { flags: ["--foo_flags"] },
	include_build_directory: false,
	bazel_module: { bp2build_available: true },
}`,
		ExpectedBazelTargets: append([]string{
			MakeBazelTarget("genlex", "foo_lib_genlex_l", AttrNameToString{
				"srcs": `[
        "foo1.l",
        "foo2.l",
    ]`,
				"lexopts": `["--foo_flags"]`,
			}),
			MakeBazelTarget("genlex", "foo_lib_genlex_ll", AttrNameToString{
				"srcs": `[
        "bar1.ll",
        "bar2.ll",
    ]`,
				"lexopts": `["--foo_flags"]`,
			}),
		},
			makeCcLibraryTargets("foo_lib", AttrNameToString{
				"srcs": `[
        "bar.cc",
        ":foo_lib_genlex_ll",
    ]`,
				"srcs_c": `[
        "foo.c",
        ":foo_lib_genlex_l",
    ]`,
			})...),
	})
}

func TestCCLibraryRuntimeDeps(t *testing.T) {
	runCcLibrarySharedTestCase(t, Bp2buildTestCase{
		Blueprint: `cc_library_shared {
	name: "bar",
}

cc_library {
  name: "foo",
  runtime_libs: ["bar"],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_shared", "bar", AttrNameToString{
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"runtime_deps":   `[":bar"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"runtime_deps":   `[":bar"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithInstructionSet(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `cc_library {
    name: "foo",
    arch: {
      arm: {
        instruction_set: "arm",
      }
    }
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("foo", AttrNameToString{
			"features": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["arm_isa_arm"],
        "//conditions:default": [],
    })`,
			"local_includes": `["."]`,
		}),
	})
}

func TestCcLibraryEmptySuffix(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with empty suffix",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"foo.c": "",
		},
		Blueprint: `cc_library {
    name: "foo",
    suffix: "",
    srcs: ["foo.c"],
    include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"srcs_c": `["foo.c"]`,
				"suffix": `""`,
			}),
		},
	})
}

func TestCcLibrarySuffix(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with suffix",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"foo.c": "",
		},
		Blueprint: `cc_library {
    name: "foo",
    suffix: "-suf",
    srcs: ["foo.c"],
    include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"srcs_c": `["foo.c"]`,
				"suffix": `"-suf"`,
			}),
		},
	})
}

func TestCcLibraryArchVariantSuffix(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with arch-variant suffix",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Filesystem: map[string]string{
			"foo.c": "",
		},
		Blueprint: `cc_library {
    name: "foo",
    arch: {
        arm64: { suffix: "-64" },
        arm:   { suffix: "-32" },
		},
    srcs: ["foo.c"],
    include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"srcs_c": `["foo.c"]`,
				"suffix": `select({
        "//build/bazel_common_rules/platforms/arch:arm": "-32",
        "//build/bazel_common_rules/platforms/arch:arm64": "-64",
        "//conditions:default": None,
    })`,
			}),
		},
	})
}

func TestCcLibraryWithAidlLibrary(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with aidl_library",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
aidl_library {
    name: "A_aidl",
    srcs: ["aidl/A.aidl"],
	hdrs: ["aidl/Header.aidl"],
	strip_import_prefix: "aidl",
}
cc_library {
	name: "foo",
	aidl: {
		libs: ["A_aidl"],
	},
	export_include_dirs: ["include"],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("aidl_library", "A_aidl", AttrNameToString{
				"srcs":                `["aidl/A.aidl"]`,
				"hdrs":                `["aidl/Header.aidl"]`,
				"strip_import_prefix": `"aidl"`,
				"tags":                `["apex_available=//apex_available:anyapex"]`,
			}),
			MakeBazelTarget("cc_aidl_library", "foo_cc_aidl_library", AttrNameToString{
				"deps":            `[":A_aidl"]`,
				"local_includes":  `["."]`,
				"export_includes": `["include"]`,
			}),
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
				"local_includes":                    `["."]`,
				"export_includes":                   `["include"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
				"local_includes":                    `["."]`,
				"export_includes":                   `["include"]`,
			}),
		},
	})
}

func TestCcLibraryWithAidlSrcs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with aidl srcs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
filegroup {
    name: "A_aidl",
    srcs: ["aidl/A.aidl"],
	path: "aidl",
}
cc_library {
	name: "foo",
	srcs: [
		":A_aidl",
		"B.aidl",
	],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("aidl_library", "A_aidl", AttrNameToString{
				"srcs":                `["aidl/A.aidl"]`,
				"strip_import_prefix": `"aidl"`,
				"tags":                `["apex_available=//apex_available:anyapex"]`,
			}),
			MakeBazelTarget("aidl_library", "foo_aidl_library", AttrNameToString{
				"srcs": `["B.aidl"]`,
			}),
			MakeBazelTarget("cc_aidl_library", "foo_cc_aidl_library", AttrNameToString{
				"local_includes": `["."]`,
				"deps": `[
        ":A_aidl",
        ":foo_aidl_library",
    ]`,
			}),
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
				"local_includes":                    `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
				"local_includes":                    `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithNonAdjacentAidlFilegroup(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with non aidl filegroup",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"//path/to/A:A_aidl"},
		Filesystem: map[string]string{
			"path/to/A/Android.bp": `
filegroup {
    name: "A_aidl",
    srcs: ["aidl/A.aidl"],
    path: "aidl",
}`,
		},
		Blueprint: `
cc_library {
    name: "foo",
    srcs: [
        ":A_aidl",
    ],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_aidl_library", "foo_cc_aidl_library", AttrNameToString{
				"local_includes": `["."]`,
				"deps":           `["//path/to/A:A_aidl"]`,
			}),
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
				"local_includes":                    `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"local_includes":                    `["."]`,
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
			}),
		},
	})
}

func TestCcLibraryWithExportAidlHeaders(t *testing.T) {
	t.Parallel()

	expectedBazelTargets := []string{
		MakeBazelTarget("cc_aidl_library", "foo_cc_aidl_library", AttrNameToString{
			"local_includes": `["."]`,
			"deps":           `[":foo_aidl_library"]`,
		}),
		MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
			"whole_archive_deps": `[":foo_cc_aidl_library"]`,
			"local_includes":     `["."]`,
		}),
		MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
			"whole_archive_deps": `[":foo_cc_aidl_library"]`,
			"local_includes":     `["."]`,
		}),
	}
	testCases := []struct {
		description          string
		bp                   string
		expectedBazelTargets []string
	}{
		{
			description: "cc_library with aidl srcs and aidl.export_aidl_headers set",
			bp: `
			cc_library {
				name: "foo",
				srcs: [
					"Foo.aidl",
				],
				aidl: {
					export_aidl_headers: true,
				}
			}`,
			expectedBazelTargets: append(
				expectedBazelTargets,
				MakeBazelTarget("aidl_library", "foo_aidl_library", AttrNameToString{
					"srcs": `["Foo.aidl"]`,
				})),
		},
		{
			description: "cc_library with aidl.libs and aidl.export_aidl_headers set",
			bp: `
			aidl_library {
				name: "foo_aidl_library",
				srcs: ["Foo.aidl"],
			}
			cc_library {
				name: "foo",
				aidl: {
					libs: ["foo_aidl_library"],
					export_aidl_headers: true,
				}
			}`,
			expectedBazelTargets: append(
				expectedBazelTargets,
				MakeBazelTargetNoRestrictions("aidl_library", "foo_aidl_library", AttrNameToString{
					"srcs": `["Foo.aidl"]`,
					"tags": `["apex_available=//apex_available:anyapex"]`,
				}),
			),
		},
	}

	for _, testCase := range testCases {
		runCcLibraryTestCase(t, Bp2buildTestCase{
			Description:                "cc_library with export aidl headers",
			ModuleTypeUnderTest:        "cc_library",
			ModuleTypeUnderTestFactory: cc.LibraryFactory,
			Blueprint:                  testCase.bp,
			ExpectedBazelTargets:       testCase.expectedBazelTargets,
		})
	}
}

func TestCcLibraryWithTargetApex(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with target.apex",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"bar", "baz", "buh"},
		Blueprint: `
cc_library {
    name: "foo",
	shared_libs: ["bar", "baz"],
	static_libs: ["baz", "buh"],
	target: {
        apex: {
            exclude_shared_libs: ["bar"],
            exclude_static_libs: ["buh"],
        }
    }
}` + simpleModule("cc_library_static", "baz") +
			simpleModule("cc_library_static", "buh") +
			simpleModule("cc_library_static", "bar"),
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_deps": `[":baz"] + select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":buh"],
    })`,
				"implementation_dynamic_deps": `[":baz"] + select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":bar"],
    })`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"implementation_deps": `[":baz"] + select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":buh"],
    })`,
				"implementation_dynamic_deps": `[":baz"] + select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":bar"],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithTargetApexAndExportLibHeaders(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with target.apex and export_shared|static_lib_headers",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library_static {
    name: "foo",
	shared_libs: ["bar", "baz"],
    static_libs: ["abc"],
    export_shared_lib_headers: ["baz"],
    export_static_lib_headers: ["abc"],
	target: {
        apex: {
            exclude_shared_libs: ["baz", "bar"],
            exclude_static_libs: ["abc"],
        }
    }
}` + simpleModule("cc_library_static", "bar") +
			simpleModule("cc_library_static", "baz") +
			simpleModule("cc_library_static", "abc"),
		StubbedBuildDefinitions: []string{"bar", "baz", "abc"},
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
				"implementation_dynamic_deps": `select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":bar"],
    })`,
				"dynamic_deps": `select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":baz"],
    })`,
				"deps": `select({
        "//build/bazel/rules/apex:in_apex": [],
        "//conditions:default": [":abc"],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithSyspropSrcs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with sysprop sources",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	srcs: [
		"bar.sysprop",
		"baz.sysprop",
		"blah.cpp",
	],
	min_sdk_version: "5",
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("sysprop_library", "foo_sysprop_library", AttrNameToString{
				"srcs": `[
        "bar.sysprop",
        "baz.sysprop",
    ]`,
			}),
			MakeBazelTarget("cc_sysprop_library_static", "foo_cc_sysprop_library_static", AttrNameToString{
				"dep":             `":foo_sysprop_library"`,
				"min_sdk_version": `"5"`,
			}),
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"srcs":               `["blah.cpp"]`,
				"local_includes":     `["."]`,
				"min_sdk_version":    `"5"`,
				"whole_archive_deps": `[":foo_cc_sysprop_library_static"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"srcs":               `["blah.cpp"]`,
				"local_includes":     `["."]`,
				"min_sdk_version":    `"5"`,
				"whole_archive_deps": `[":foo_cc_sysprop_library_static"]`,
			}),
		},
	})
}

func TestCcLibraryWithSyspropSrcsSomeConfigs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with sysprop sources in some configs but not others",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	host_supported: true,
	srcs: [
		"blah.cpp",
	],
	target: {
		android: {
			srcs: ["bar.sysprop"],
		},
	},
	min_sdk_version: "5",
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("sysprop_library", "foo_sysprop_library", AttrNameToString{
				"srcs": `select({
        "//build/bazel_common_rules/platforms/os:android": ["bar.sysprop"],
        "//conditions:default": [],
    })`,
			}),
			MakeBazelTargetNoRestrictions("cc_sysprop_library_static", "foo_cc_sysprop_library_static", AttrNameToString{
				"dep":             `":foo_sysprop_library"`,
				"min_sdk_version": `"5"`,
			}),
			MakeBazelTargetNoRestrictions("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"srcs":            `["blah.cpp"]`,
				"local_includes":  `["."]`,
				"min_sdk_version": `"5"`,
				"whole_archive_deps": `select({
        "//build/bazel_common_rules/platforms/os:android": [":foo_cc_sysprop_library_static"],
        "//conditions:default": [],
    })`,
			}),
			MakeBazelTargetNoRestrictions("cc_library_shared", "foo", AttrNameToString{
				"srcs":            `["blah.cpp"]`,
				"local_includes":  `["."]`,
				"min_sdk_version": `"5"`,
				"whole_archive_deps": `select({
        "//build/bazel_common_rules/platforms/os:android": [":foo_cc_sysprop_library_static"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestCcLibraryWithAidlAndLibs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_aidl_library depends on libs from parent cc_library_static",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"bar-static", "baz-static", "bar-shared", "baz-shared"},
		Blueprint: `
cc_library_static {
	name: "foo",
	srcs: [
		"Foo.aidl",
	],
	static_libs: [
		"bar-static",
		"baz-static",
	],
	shared_libs: [
		"bar-shared",
		"baz-shared",
	],
	export_static_lib_headers: [
		"baz-static",
	],
	export_shared_lib_headers: [
		"baz-shared",
	],
}` +
			simpleModule("cc_library_static", "bar-static") +
			simpleModule("cc_library_static", "baz-static") +
			simpleModule("cc_library", "bar-shared") +
			simpleModule("cc_library", "baz-shared"),
		ExpectedBazelTargets: []string{
			MakeBazelTarget("aidl_library", "foo_aidl_library", AttrNameToString{
				"srcs": `["Foo.aidl"]`,
			}),
			MakeBazelTarget("cc_aidl_library", "foo_cc_aidl_library", AttrNameToString{
				"local_includes": `["."]`,
				"deps":           `[":foo_aidl_library"]`,
				"implementation_deps": `[
        ":baz-static",
        ":bar-static",
    ]`,
				"implementation_dynamic_deps": `[
        ":baz-shared",
        ":bar-shared",
    ]`,
			}),
			MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
				"implementation_whole_archive_deps": `[":foo_cc_aidl_library"]`,
				"deps":                              `[":baz-static"]`,
				"implementation_deps":               `[":bar-static"]`,
				"dynamic_deps":                      `[":baz-shared"]`,
				"implementation_dynamic_deps":       `[":bar-shared"]`,
				"local_includes":                    `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithTidy(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library uses tidy properties",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library_static {
	name: "foo",
	srcs: ["foo.cpp"],
}
cc_library_static {
	name: "foo-no-tidy",
	srcs: ["foo.cpp"],
	tidy: false,
}
cc_library_static {
	name: "foo-tidy",
	srcs: ["foo.cpp"],
	tidy: true,
	tidy_checks: ["check1", "check2"],
	tidy_checks_as_errors: ["check1error", "check2error"],
	tidy_disabled_srcs: ["bar.cpp"],
	tidy_timeout_srcs: ["baz.cpp"],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
				"local_includes": `["."]`,
				"srcs":           `["foo.cpp"]`,
			}),
			MakeBazelTarget("cc_library_static", "foo-no-tidy", AttrNameToString{
				"local_includes": `["."]`,
				"srcs":           `["foo.cpp"]`,
				"tidy":           `"never"`,
			}),
			MakeBazelTarget("cc_library_static", "foo-tidy", AttrNameToString{
				"local_includes": `["."]`,
				"srcs":           `["foo.cpp"]`,
				"tidy":           `"local"`,
				"tidy_checks": `[
        "check1",
        "check2",
    ]`,
				"tidy_checks_as_errors": `[
        "check1error",
        "check2error",
    ]`,
				"tidy_disabled_srcs": `["bar.cpp"]`,
				"tidy_timeout_srcs":  `["baz.cpp"]`,
			}),
		},
	})
}

func TestCcLibraryWithAfdoEnabled(t *testing.T) {
	bp := `
cc_library {
	name: "foo",
	afdo: true,
	include_build_directory: false,
}`

	// TODO(b/260714900): Add test case for arch-specific afdo profile
	testCases := []struct {
		description          string
		filesystem           map[string]string
		expectedBazelTargets []string
	}{
		{
			description: "cc_library with afdo enabled and existing profile",
			filesystem: map[string]string{
				"vendor/google_data/pgo_profile/sampling/Android.bp": "",
				"vendor/google_data/pgo_profile/sampling/foo.afdo":   "",
			},
			expectedBazelTargets: []string{
				MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{}),
				MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
					"fdo_profile": `"//vendor/google_data/pgo_profile/sampling:foo"`,
				}),
			},
		},
		{
			description: "cc_library with afdo enabled and existing profile in AOSP",
			filesystem: map[string]string{
				"toolchain/pgo-profiles/sampling/Android.bp": "",
				"toolchain/pgo-profiles/sampling/foo.afdo":   "",
			},
			expectedBazelTargets: []string{
				MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{}),
				MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
					"fdo_profile": `"//toolchain/pgo-profiles/sampling:foo"`,
				}),
			},
		},
		{
			description: "cc_library with afdo enabled but profile filename doesn't match with module name",
			filesystem: map[string]string{
				"toolchain/pgo-profiles/sampling/Android.bp": "",
				"toolchain/pgo-profiles/sampling/bar.afdo":   "",
			},
			expectedBazelTargets: []string{
				MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{}),
				MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{}),
			},
		},
		{
			description: "cc_library with afdo enabled but profile doesn't exist",
			expectedBazelTargets: []string{
				MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{}),
				MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{}),
			},
		},
		{
			description: "cc_library with afdo enabled and existing profile but BUILD file doesn't exist",
			filesystem: map[string]string{
				"vendor/google_data/pgo_profile/sampling/foo.afdo": "",
			},
			expectedBazelTargets: []string{
				MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{}),
				MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{}),
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			runCcLibraryTestCase(t, Bp2buildTestCase{
				ExpectedBazelTargets:       testCase.expectedBazelTargets,
				ModuleTypeUnderTest:        "cc_library",
				ModuleTypeUnderTestFactory: cc.LibraryFactory,
				Description:                testCase.description,
				Blueprint:                  binaryReplacer.Replace(bp),
				Filesystem:                 testCase.filesystem,
			})
		})
	}
}

func TestCcLibraryHeaderAbiChecker(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with header abi checker",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `cc_library {
    name: "foo",
    header_abi_checker: {
        enabled: true,
        symbol_file: "a.map.txt",
        exclude_symbol_versions: [
						"29",
						"30",
				],
        exclude_symbol_tags: [
						"tag1",
						"tag2",
				],
        check_all_apis: true,
        diff_flags: ["-allow-adding-removing-weak-symbols"],
    },
    include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"abi_checker_enabled":     `True`,
				"abi_checker_symbol_file": `"a.map.txt"`,
				"abi_checker_exclude_symbol_versions": `[
        "29",
        "30",
    ]`,
				"abi_checker_exclude_symbol_tags": `[
        "tag1",
        "tag2",
    ]`,
				"abi_checker_check_all_apis": `True`,
				"abi_checker_diff_flags":     `["-allow-adding-removing-weak-symbols"]`,
			}),
		},
	})
}

func TestCcLibraryApexAvailable(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library apex_available converted to tags",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "a",
    srcs: ["a.cpp"],
    apex_available: ["com.android.foo"],
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"tags":           `["apex_available=com.android.foo"]`,
			"srcs":           `["a.cpp"]`,
			"local_includes": `["."]`,
		}),
	},
	)
}

func TestCcLibraryApexAvailableMultiple(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library apex_available converted to multiple tags",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "a",
    srcs: ["a.cpp"],
    apex_available: ["com.android.foo", "//apex_available:platform", "com.android.bar"],
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"tags": `[
        "apex_available=com.android.foo",
        "apex_available=//apex_available:platform",
        "apex_available=com.android.bar",
    ]`,
			"srcs":           `["a.cpp"]`,
			"local_includes": `["."]`,
		}),
	},
	)
}

// Export_include_dirs and Export_system_include_dirs have "variant_prepend" tag.
// In bp2build output, variant info(select) should go before general info.
// Internal order of the property should be unchanged. (e.g. ["eid1", "eid2"])
func TestCcLibraryVariantPrependPropOrder(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library variant prepend properties order",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library {
  name: "a",
  srcs: ["a.cpp"],
  export_include_dirs: ["eid1", "eid2"],
  export_system_include_dirs: ["esid1", "esid2"],
    target: {
      android: {
        export_include_dirs: ["android_eid1", "android_eid2"],
        export_system_include_dirs: ["android_esid1", "android_esid2"],
      },
      android_arm: {
        export_include_dirs: ["android_arm_eid1", "android_arm_eid2"],
        export_system_include_dirs: ["android_arm_esid1", "android_arm_esid2"],
      },
      linux: {
        export_include_dirs: ["linux_eid1", "linux_eid2"],
        export_system_include_dirs: ["linux_esid1", "linux_esid2"],
      },
    },
    multilib: {
      lib32: {
        export_include_dirs: ["lib32_eid1", "lib32_eid2"],
        export_system_include_dirs: ["lib32_esid1", "lib32_esid2"],
      },
    },
    arch: {
      arm: {
        export_include_dirs: ["arm_eid1", "arm_eid2"],
        export_system_include_dirs: ["arm_esid1", "arm_esid2"],
      },
    }
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"export_includes": `select({
        "//build/bazel_common_rules/platforms/os_arch:android_arm": [
            "android_arm_eid1",
            "android_arm_eid2",
        ],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": [
            "android_eid1",
            "android_eid2",
            "linux_eid1",
            "linux_eid2",
        ],
        "//build/bazel_common_rules/platforms/os:linux_bionic": [
            "linux_eid1",
            "linux_eid2",
        ],
        "//build/bazel_common_rules/platforms/os:linux_glibc": [
            "linux_eid1",
            "linux_eid2",
        ],
        "//build/bazel_common_rules/platforms/os:linux_musl": [
            "linux_eid1",
            "linux_eid2",
        ],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/arch:arm": [
            "lib32_eid1",
            "lib32_eid2",
            "arm_eid1",
            "arm_eid2",
        ],
        "//build/bazel_common_rules/platforms/arch:x86": [
            "lib32_eid1",
            "lib32_eid2",
        ],
        "//conditions:default": [],
    }) + [
        "eid1",
        "eid2",
    ]`,
			"export_system_includes": `select({
        "//build/bazel_common_rules/platforms/os_arch:android_arm": [
            "android_arm_esid1",
            "android_arm_esid2",
        ],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": [
            "android_esid1",
            "android_esid2",
            "linux_esid1",
            "linux_esid2",
        ],
        "//build/bazel_common_rules/platforms/os:linux_bionic": [
            "linux_esid1",
            "linux_esid2",
        ],
        "//build/bazel_common_rules/platforms/os:linux_glibc": [
            "linux_esid1",
            "linux_esid2",
        ],
        "//build/bazel_common_rules/platforms/os:linux_musl": [
            "linux_esid1",
            "linux_esid2",
        ],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/arch:arm": [
            "lib32_esid1",
            "lib32_esid2",
            "arm_esid1",
            "arm_esid2",
        ],
        "//build/bazel_common_rules/platforms/arch:x86": [
            "lib32_esid1",
            "lib32_esid2",
        ],
        "//conditions:default": [],
    }) + [
        "esid1",
        "esid2",
    ]`,
			"srcs":                   `["a.cpp"]`,
			"local_includes":         `["."]`,
			"target_compatible_with": `["//build/bazel_common_rules/platforms/os:android"]`,
		}),
	},
	)
}

func TestCcLibraryWithIntegerOverflowProperty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when integer_overflow property is provided",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
		name: "foo",
		sanitize: {
				integer_overflow: true,
		},
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["ubsan_integer_overflow"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["ubsan_integer_overflow"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithMiscUndefinedProperty(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when misc_undefined property is provided",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
		name: "foo",
		sanitize: {
				misc_undefined: ["undefined", "nullability"],
		},
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `[
        "ubsan_undefined",
        "ubsan_nullability",
    ]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features": `[
        "ubsan_undefined",
        "ubsan_nullability",
    ]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithSanitizerBlocklist(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct feature when sanitize.blocklist is provided",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
		name: "foo",
		sanitize: {
			blocklist: "foo_blocklist.txt",
		},
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"copts": `select({
        "//build/bazel/rules/cc:sanitizers_enabled": ["-fsanitize-ignorelist=$(location foo_blocklist.txt)"],
        "//conditions:default": [],
    })`,
				"additional_compiler_inputs": `select({
        "//build/bazel/rules/cc:sanitizers_enabled": [":foo_blocklist.txt"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"copts": `select({
        "//build/bazel/rules/cc:sanitizers_enabled": ["-fsanitize-ignorelist=$(location foo_blocklist.txt)"],
        "//conditions:default": [],
    })`,
				"additional_compiler_inputs": `select({
        "//build/bazel/rules/cc:sanitizers_enabled": [":foo_blocklist.txt"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithUBSanPropertiesArchSpecific(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct feature select when UBSan props are specified in arch specific blocks",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
		name: "foo",
		sanitize: {
				misc_undefined: ["undefined", "nullability"],
		},
		target: {
				android: {
						sanitize: {
								misc_undefined: ["alignment"],
						},
				},
				linux_glibc: {
						sanitize: {
								integer_overflow: true,
						},
				},
		},
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `[
        "ubsan_undefined",
        "ubsan_nullability",
    ] + select({
        "//build/bazel_common_rules/platforms/os:android": ["ubsan_alignment"],
        "//build/bazel_common_rules/platforms/os:linux_glibc": ["ubsan_integer_overflow"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features": `[
        "ubsan_undefined",
        "ubsan_nullability",
    ] + select({
        "//build/bazel_common_rules/platforms/os:android": ["ubsan_alignment"],
        "//build/bazel_common_rules/platforms/os:linux_glibc": ["ubsan_integer_overflow"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryInApexWithStubSharedLibs(t *testing.T) {
	runCcLibrarySharedTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with in apex with stub shared_libs and export_shared_lib_headers",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"barlib", "bazlib"},
		Blueprint: `
cc_library {
	name: "barlib",
	stubs: { symbol_file: "bar.map.txt", versions: ["28", "29", "current"] },
	apex_available: ["//apex_available:platform",],
}
cc_library {
	name: "bazlib",
	stubs: { symbol_file: "bar.map.txt", versions: ["28", "29", "current"] },
	apex_available: ["//apex_available:platform",],
}
cc_library {
    name: "foo",
	  shared_libs: ["barlib", "bazlib"],
    export_shared_lib_headers: ["bazlib"],
    apex_available: [
        "//apex_available:platform",
    ],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"implementation_dynamic_deps": `[":barlib"]`,
				"dynamic_deps":                `[":bazlib"]`,
				"local_includes":              `["."]`,
				"tags":                        `["apex_available=//apex_available:platform"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"implementation_dynamic_deps": `[":barlib"]`,
				"dynamic_deps":                `[":bazlib"]`,
				"local_includes":              `["."]`,
				"tags":                        `["apex_available=//apex_available:platform"]`,
			}),
		},
	})
}

func TestCcLibraryWithThinLto(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when thin LTO is enabled",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	lto: {
		thin: true,
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["android_thin_lto"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["android_thin_lto"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithLtoNever(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when LTO is explicitly disabled",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	lto: {
		never: true,
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["-android_thin_lto"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["-android_thin_lto"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithThinLtoArchSpecific(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when LTO differs across arch and os variants",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	target: {
		android: {
			lto: {
				thin: true,
			},
		},
	},
	arch: {
		riscv64: {
			lto: {
				thin: false,
			},
		},
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
				"features": `select({
        "//build/bazel_common_rules/platforms/os_arch:android_arm": ["android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_arm64": ["android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_riscv64": ["-android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_x86": ["android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_x86_64": ["android_thin_lto"],
        "//conditions:default": [],
    })`}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"local_includes": `["."]`,
				"features": `select({
        "//build/bazel_common_rules/platforms/os_arch:android_arm": ["android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_arm64": ["android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_riscv64": ["-android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_x86": ["android_thin_lto"],
        "//build/bazel_common_rules/platforms/os_arch:android_x86_64": ["android_thin_lto"],
        "//conditions:default": [],
    })`}),
		},
	})
}

func TestCcLibraryWithThinLtoDisabledDefaultEnabledVariant(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when LTO disabled by default but enabled on a particular variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	lto: {
		never: true,
	},
	target: {
		android: {
			lto: {
				thin: true,
				never: false,
			},
		},
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
				"features": `select({
        "//build/bazel_common_rules/platforms/os:android": ["android_thin_lto"],
        "//conditions:default": ["-android_thin_lto"],
    })`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"local_includes": `["."]`,
				"features": `select({
        "//build/bazel_common_rules/platforms/os:android": ["android_thin_lto"],
        "//conditions:default": ["-android_thin_lto"],
    })`,
			}),
		},
	})
}

func TestCcLibraryWithThinLtoWholeProgramVtables(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when thin LTO is enabled with whole_program_vtables",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	lto: {
		thin: true,
	},
	whole_program_vtables: true,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `[
        "android_thin_lto",
        "android_thin_lto_whole_program_vtables",
    ]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features": `[
        "android_thin_lto",
        "android_thin_lto_whole_program_vtables",
    ]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryHiddenVisibilityConvertedToFeature(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library changes hidden visibility flag to feature",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	cflags: ["-fvisibility=hidden"],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["visibility_hidden"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["visibility_hidden"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryHiddenVisibilityConvertedToFeatureSharedSpecific(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library changes hidden visibility flag to feature when specific to shared variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	shared: {
		cflags: ["-fvisibility=hidden"],
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["visibility_hidden"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryHiddenVisibilityConvertedToFeatureStaticSpecific(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library changes hidden visibility flag to feature when specific to static variant",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	static: {
		cflags: ["-fvisibility=hidden"],
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["visibility_hidden"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryHiddenVisibilityConvertedToFeatureOsSpecific(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library changes hidden visibility flag to feature when specific to an os",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	target: {
		android: {
			cflags: ["-fvisibility=hidden"],
		},
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `select({
        "//build/bazel_common_rules/platforms/os:android": ["visibility_hidden"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features": `select({
        "//build/bazel_common_rules/platforms/os:android": ["visibility_hidden"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

// Test that a config_setting specific to an apex is created by cc_library.
func TestCcLibraryCreatesInApexConfigSetting(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library creates a config_setting for each apex in apex_available",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Dir:                        "build/bazel/rules/apex",
		Blueprint: `
cc_library {
	name: "foo",
	apex_available: [
	"//apex_available:platform", // This will be skipped, since it is equivalent to //build/bazel/rules/apex:android-non_apex
	"myapex"
	],
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions(
				"config_setting",
				"myapex",
				AttrNameToString{
					"flag_values": `{
        "//build/bazel/rules/apex:api_domain": "myapex",
    }`,
					"constraint_values": `["//build/bazel_common_rules/platforms/os:android"]`,
				},
			),
		},
	})
}

func TestCcLibraryCppFlagsInProductVariables(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library cppflags in product variables",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: soongCcLibraryPreamble + `cc_library {
    name: "a",
    srcs: ["a.cpp"],
    cppflags: [
        "-Wextra",
        "-DDEBUG_ONLY_CODE=0",
    ],
    product_variables: {
        eng: {
            cppflags: [
                "-UDEBUG_ONLY_CODE",
                "-DDEBUG_ONLY_CODE=1",
            ],
        },
    },
    include_build_directory: false,
}
`,
		ExpectedBazelTargets: makeCcLibraryTargets("a", AttrNameToString{
			"cppflags": `[
        "-Wextra",
        "-DDEBUG_ONLY_CODE=0",
    ] + select({
        "//build/bazel/product_config/config_settings:eng": [
            "-UDEBUG_ONLY_CODE",
            "-DDEBUG_ONLY_CODE=1",
        ],
        "//conditions:default": [],
    })`,
			"srcs": `["a.cpp"]`,
		}),
	},
	)
}

func TestCcLibraryYaccConversion(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library is built from .y/.yy files",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		StubbedBuildDefinitions:    []string{"staticlib", "sharedlib"},
		Blueprint: soongCcLibraryPreamble + `cc_library {
    name: "a",
    srcs: [
	"a.cpp",
	"a.yy",
    ],
    shared_libs: ["sharedlib"],
    static_libs: ["staticlib"],
    yacc: {
	    flags: ["someYaccFlag"],
	    gen_location_hh: true,
	    gen_position_hh: true,
	},
}
cc_library_static {
	name: "staticlib",
}
cc_library {
	name: "sharedlib",
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_yacc_static_library", "a_yacc", AttrNameToString{
				"src":                         `"a.yy"`,
				"implementation_deps":         `[":staticlib"]`,
				"implementation_dynamic_deps": `[":sharedlib"]`,
				"flags":                       `["someYaccFlag"]`,
				"gen_location_hh":             "True",
				"gen_position_hh":             "True",
				"local_includes":              `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "a", AttrNameToString{
				"srcs":                              `["a.cpp"]`,
				"implementation_deps":               `[":staticlib"]`,
				"implementation_dynamic_deps":       `[":sharedlib"]`,
				"implementation_whole_archive_deps": `[":a_yacc"]`,
				"local_includes":                    `["."]`,
			}),
			MakeBazelTarget("cc_library_static", "a_bp2build_cc_library_static", AttrNameToString{
				"srcs":                              `["a.cpp"]`,
				"implementation_deps":               `[":staticlib"]`,
				"implementation_dynamic_deps":       `[":sharedlib"]`,
				"implementation_whole_archive_deps": `[":a_yacc"]`,
				"local_includes":                    `["."]`,
			}),
		},
	})
}

func TestCcLibraryHostLdLibs(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_binary linker flags for host_ldlibs",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint: soongCcLibraryPreamble + `cc_binary {
    name: "a",
    host_supported: true,
    ldflags: ["-lcommon"],
    target: {
	linux: {
		host_ldlibs: [
			"-llinux",
		],
	},
	darwin: {
		ldflags: ["-ldarwinadditional"],
		host_ldlibs: [
			"-ldarwin",
		],
	},
	windows: {
		host_ldlibs: [
			"-lwindows",
		],
	},
    },
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("cc_binary", "a", AttrNameToString{
				"linkopts": `["-lcommon"] + select({
        "//build/bazel_common_rules/platforms/os:darwin": [
            "-ldarwinadditional",
            "-ldarwin",
        ],
        "//build/bazel_common_rules/platforms/os:linux_glibc": ["-llinux"],
        "//build/bazel_common_rules/platforms/os:windows": ["-lwindows"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithCfi(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when cfi is enabled",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	sanitize: {
		cfi: true,
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["android_cfi"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["android_cfi"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithCfiOsSpecific(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when cfi is enabled for specific variants",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	target: {
		android: {
			sanitize: {
				cfi: true,
			},
		},
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `select({
        "//build/bazel_common_rules/platforms/os:android": ["android_cfi"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features": `select({
        "//build/bazel_common_rules/platforms/os:android": ["android_cfi"],
        "//conditions:default": [],
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithCfiAndCfiAssemblySupport(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library has correct features when cfi is enabled with cfi_assembly_support",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	sanitize: {
		cfi: true,
		config: {
			cfi_assembly_support: true,
		},
	},
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features": `[
        "android_cfi",
        "android_cfi_assembly_support",
    ]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features": `[
        "android_cfi",
        "android_cfi_assembly_support",
    ]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryExplicitlyDisablesCfiWhenFalse(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library disables cfi when explciitly set to false in the bp",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "foo",
	sanitize: {
		cfi: false,
	},
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_bp2build_cc_library_static", AttrNameToString{
				"features":       `["-android_cfi"]`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo", AttrNameToString{
				"features":       `["-android_cfi"]`,
				"local_includes": `["."]`,
			}),
		},
	})
}

func TestCcLibraryWithStem(t *testing.T) {
	runCcLibraryTestCase(t, Bp2buildTestCase{
		Description:                "cc_library with stem property",
		ModuleTypeUnderTest:        "cc_library_shared",
		ModuleTypeUnderTestFactory: cc.LibrarySharedFactory,
		Blueprint: soongCcLibraryPreamble + `
cc_library_shared {
	name: "foo_with_stem_simple",
	stem: "foo",
}
cc_library_shared {
	name: "foo_with_arch_variant_stem",
	arch: {
		arm: {
			stem: "foo-arm",
		},
		arm64: {
			stem: "foo-arm64",
		},
	},
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_shared", "foo_with_stem_simple", AttrNameToString{
				"stem":           `"foo"`,
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo_with_arch_variant_stem", AttrNameToString{
				"stem": `select({
        "//build/bazel_common_rules/platforms/arch:arm": "foo-arm",
        "//build/bazel_common_rules/platforms/arch:arm64": "foo-arm64",
        "//conditions:default": None,
    })`,
				"local_includes": `["."]`,
			}),
		},
	})
}

// Bazel enforces that proto_library and the .proto file are in the same bazel package
func TestGenerateProtoLibraryInSamePackage(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "cc_library depends on .proto files from multiple packages",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library_static {
	name: "foo",
	srcs: [
	   "foo.proto",
	   "bar/bar.proto", // Different package because there is a bar/Android.bp
	   "baz/subbaz/baz.proto", // Different package because there is baz/subbaz/Android.bp
	],
	proto: {
		canonical_path_from_root: true,
	}
}
` + simpleModule("cc_library", "libprotobuf-cpp-lite"),
		Filesystem: map[string]string{
			"bar/Android.bp":        "",
			"baz/subbaz/Android.bp": "",
		},
	}

	// We will run the test 3 times and check in the root, bar and baz/subbaz directories
	// Root dir
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
			"local_includes":                    `["."]`,
			"deps":                              `[":libprotobuf-cpp-lite"]`,
			"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
		}),
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs": `["foo.proto"]`,
			"tags": `["manual"]`,
		}),
		MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
			"deps": `[
        ":foo_proto",
        "//bar:foo_proto",
        "//baz/subbaz:foo_proto",
    ]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// bar dir
	tc.Dir = "bar"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs": `["//bar:bar.proto"]`,
			"tags": `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// baz/subbaz dir
	tc.Dir = "baz/subbaz"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs": `["//baz/subbaz:baz.proto"]`,
			"tags": `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)
}

// Bazel enforces that proto_library and the .proto file are in the same bazel package
func TestGenerateProtoLibraryInSamePackageNotCanonicalFromRoot(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "cc_library depends on .proto files from multiple packages",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library_static {
	name: "foo",
	srcs: [
	   "foo.proto",
	   "bar/bar.proto", // Different package because there is a bar/Android.bp
	   "baz/subbaz/baz.proto", // Different package because there is baz/subbaz/Android.bp
	],
	proto: {
		canonical_path_from_root: false,
	}
}
` + simpleModule("cc_library", "libprotobuf-cpp-lite"),
		Filesystem: map[string]string{
			"bar/Android.bp":        "",
			"baz/subbaz/Android.bp": "",
		},
	}

	// We will run the test 3 times and check in the root, bar and baz/subbaz directories
	// Root dir
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
			"local_includes":                    `["."]`,
			"deps":                              `[":libprotobuf-cpp-lite"]`,
			"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
		}),
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs":                `["foo.proto"]`,
			"strip_import_prefix": `""`,
			"tags":                `["manual"]`,
		}),
		MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
			"deps": `[
        ":foo_proto",
        "//bar:foo_proto",
        "//baz/subbaz:foo_proto",
    ]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// bar dir
	tc.Dir = "bar"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs":                `["//bar:bar.proto"]`,
			"strip_import_prefix": `""`,
			"import_prefix":       `"bar"`,
			"tags":                `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// baz/subbaz dir
	tc.Dir = "baz/subbaz"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs":                `["//baz/subbaz:baz.proto"]`,
			"strip_import_prefix": `""`,
			"import_prefix":       `"baz/subbaz"`,
			"tags":                `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)
}

func TestProtoIncludeDirs(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "cc_library depends on .proto files using proto.include_dirs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library_static {
	name: "foo",
	srcs: [
	   "foo.proto",
	],
	proto: {
		include_dirs: ["bar"],
	}
}
` + simpleModule("cc_library", "libprotobuf-cpp-lite"),
		Filesystem: map[string]string{
			"bar/Android.bp":     "",
			"bar/bar.proto":      "",
			"bar/baz/Android.bp": "",
			"bar/baz/baz.proto":  "",
		},
	}

	// We will run the test 3 times and check in the root, bar and bar/baz directories
	// Root dir
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
			"local_includes":                    `["."]`,
			"deps":                              `[":libprotobuf-cpp-lite"]`,
			"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
		}),
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs": `["foo.proto"]`,
			"tags": `["manual"]`,
		}),
		MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
			"deps": `[":foo_proto"]`,
			"transitive_deps": `[
        "//bar:bar.include_dir_bp2build_generated_proto",
        "//bar/baz:bar.include_dir_bp2build_generated_proto",
    ]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// bar dir
	tc.Dir = "bar"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTargetNoRestrictions("proto_library", "bar.include_dir_bp2build_generated_proto", AttrNameToString{
			"srcs":                `["bar.proto"]`,
			"strip_import_prefix": `""`,
			"tags":                `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// bar/baz dir
	tc.Dir = "bar/baz"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTargetNoRestrictions("proto_library", "bar.include_dir_bp2build_generated_proto", AttrNameToString{
			"srcs":                `["//bar/baz:baz.proto"]`,
			"strip_import_prefix": `""`,
			"import_prefix":       `"baz"`,
			"tags":                `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)
}

func TestProtoIncludeDirsWithSrcsInMultiplePackages(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "cc_library has srcs in multiple bazel packages and uses proto.include_dirs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library_static {
	name: "foo",
	srcs: [
	   "foo.proto",
	   "bar/bar.proto",
	],
	proto: {
		include_dirs: ["baz"],
	}
}
` + simpleModule("cc_library", "libprotobuf-cpp-lite"),
		Filesystem: map[string]string{
			"bar/Android.bp": "", // package boundary
			"baz/Android.bp": "",
			"baz/baz.proto":  "",
		},
	}

	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
			"local_includes":                    `["."]`,
			"deps":                              `[":libprotobuf-cpp-lite"]`,
			"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
		}),
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs": `["foo.proto"]`,
			"tags": `["manual"]`,
		}),
		MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
			"deps": `[
        ":foo_proto",
        "//bar:foo_proto",
    ]`,
			"transitive_deps": `["//baz:baz.include_dir_bp2build_generated_proto"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

}

func TestProtoLocalIncludeDirs(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "cc_library depends on .proto files using proto.local_include_dirs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "libprotobuf-cpp-lite",
  // TODO: b/285631638 - A stubbed proto library dependency does not work as a protolib
  // dependency of cc_library_static.
	bazel_module: { bp2build_available: false },
}
`,
		Filesystem: map[string]string{
			"foo/Android.bp": `cc_library_static {
	name: "foo",
	srcs: [
	   "foo.proto",
	],
	proto: {
		local_include_dirs: ["foo_subdir"],
	},
	bazel_module: { bp2build_available: true },
}`,
			"foo/foo.proto":                   "",
			"foo/foo_subdir/Android.bp":       "",
			"foo/foo_subdir/foo_subdir.proto": "",
		},
	}

	// We will run the test 2 times and check in foo and foo/foo_subdir directories
	// foo dir
	tc.Dir = "foo"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTarget("cc_library_static", "foo", AttrNameToString{
			"local_includes":                    `["."]`,
			"deps":                              `["//:libprotobuf-cpp-lite"]`,
			"implementation_whole_archive_deps": `[":foo_cc_proto_lite"]`,
		}),
		MakeBazelTarget("proto_library", "foo_proto", AttrNameToString{
			"srcs": `["foo.proto"]`,
			"tags": `["manual"]`,
		}),
		MakeBazelTarget("cc_lite_proto_library", "foo_cc_proto_lite", AttrNameToString{
			"deps":            `[":foo_proto"]`,
			"transitive_deps": `["//foo/foo_subdir:foo.foo_subdir.include_dir_bp2build_generated_proto"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)

	// foo/foo_subdir
	tc.Dir = "foo/foo_subdir"
	tc.ExpectedBazelTargets = []string{
		MakeBazelTargetNoRestrictions("proto_library", "foo.foo_subdir.include_dir_bp2build_generated_proto", AttrNameToString{
			"srcs":                `["foo_subdir.proto"]`,
			"strip_import_prefix": `""`,
			"tags":                `["manual"]`,
		}),
	}
	runCcLibraryTestCase(t, tc)
}

// `foo_device` and `bar_host` can depend on .proto files of a specific dir,
// the dynamically generated proto_library should not have any target_compatible_with
func TestProtoLibraryForIncludeDirsIsOsAgnostic(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "proto_library generated for proto.include_dirs is compatible for all axes",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: simpleModule("cc_library", "libprotobuf-cpp-lite") + `
cc_library {
	name: "foo_device",
	device_supported: true, // this is the default behavior, but added explicitly here for illustration
	host_supported: false,
	proto: {include_dirs: ["dir"]},
}
cc_library {
	name: "bar_host",
	device_supported: false,
	host_supported: true,
	srcs: ["bar.proto"],
	proto: {include_dirs: ["dir"]},
}
`,
		Filesystem: map[string]string{
			"dir/Android.bp": "",
			"dir/dir.proto":  "",
		},
		Dir: "dir", // check for the generated proto_library
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("proto_library", "dir.include_dir_bp2build_generated_proto", AttrNameToString{
				"srcs":                `["dir.proto"]`,
				"strip_import_prefix": `""`,
				"tags":                `["manual"]`,
			}),
		},
	}
	runCcLibraryTestCase(t, tc)
}

func TestCcCompileMultilibConversion(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "cc_library with compile_multilib",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "lib32",
	compile_multilib: "32",
}
cc_library {
	name: "lib64",
	compile_multilib: "64",
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("cc_library_shared", "lib32", AttrNameToString{
				"local_includes": `["."]`,
				"target_compatible_with": `["//build/bazel_common_rules/platforms/os:android"] + select({
        "//build/bazel_common_rules/platforms/arch:arm64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/arch:riscv64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/arch:x86_64": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
			MakeBazelTargetNoRestrictions("cc_library_static", "lib32_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
				"target_compatible_with": `["//build/bazel_common_rules/platforms/os:android"] + select({
        "//build/bazel_common_rules/platforms/arch:arm64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/arch:riscv64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/arch:x86_64": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
			MakeBazelTargetNoRestrictions("cc_library_shared", "lib64", AttrNameToString{
				"local_includes": `["."]`,
				"target_compatible_with": `["//build/bazel_common_rules/platforms/os:android"] + select({
        "//build/bazel_common_rules/platforms/arch:arm": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/arch:x86": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
			MakeBazelTargetNoRestrictions("cc_library_static", "lib64_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
				"target_compatible_with": `["//build/bazel_common_rules/platforms/os:android"] + select({
        "//build/bazel_common_rules/platforms/arch:arm": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/arch:x86": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	}
	runCcLibraryTestCase(t, tc)
}

func TestNdkLibraryConversion(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "ndk_library conversion",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
cc_library {
	name: "libfoo",
}
ndk_library {
	name: "libfoo",
	first_version: "29",
	symbol_file: "libfoo.map.txt",
}
`,
		StubbedBuildDefinitions: []string{"libfoo"},
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_stub_suite", "libfoo.ndk_stub_libs", AttrNameToString{
				"api_surface":          `"publicapi"`,
				"included_in_ndk":      `True`,
				"soname":               `"libfoo.so"`,
				"source_library_label": `"//:libfoo"`,
				"symbol_file":          `"libfoo.map.txt"`,
				"versions": `[
        "29",
        "30",
        "S",
        "Tiramisu",
        "current",
    ]`,
			}),
		},
	}
	runCcLibraryTestCase(t, tc)
}

func TestNdkHeadersConversion(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "ndk_headers conversion",
		ModuleTypeUnderTest:        "ndk_headers",
		ModuleTypeUnderTestFactory: cc.NdkHeadersFactory,
		Blueprint: `
ndk_headers {
	name: "libfoo_headers",
	from: "from",
	to: "to",
	srcs: ["foo.h", "foo_other.h"]
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("ndk_headers", "libfoo_headers", AttrNameToString{
				"strip_import_prefix": `"from"`,
				"import_prefix":       `"to"`,
				"hdrs": `[
        "foo.h",
        "foo_other.h",
    ]`,
			}),
		},
	}
	runCcLibraryTestCase(t, tc)
}

func TestVersionedNdkHeadersConversion(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "versioned_ndk_headers conversion",
		ModuleTypeUnderTest:        "versioned_ndk_headers",
		ModuleTypeUnderTestFactory: cc.VersionedNdkHeadersFactory,
		Blueprint: `
versioned_ndk_headers {
	name: "libfoo_headers",
	from: "from",
	to: "to",
}
`,
		Filesystem: map[string]string{
			"from/foo.h":       "",
			"from/foo_other.h": "",
		},
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("ndk_headers", "libfoo_headers", AttrNameToString{
				"strip_import_prefix": `"from"`,
				"import_prefix":       `"to"`,
				"hdrs": `[
        "from/foo.h",
        "from/foo_other.h",
    ]`,
				"run_versioner": "True",
			}),
		},
	}
	runCcLibraryTestCase(t, tc)
}

// Regression test for b/303307456.
// TODO: b/202299295 - Remove this test when cc rules have proper support
// for the `required` property
func TestCcModules_requiredProperty(t *testing.T) {
	runCcLibrarySharedTestCase(t, Bp2buildTestCase{
		Description: "cc modules do not use the required property",
		Filesystem: map[string]string{
			"foo.c": "",
			"bar.c": "",
		},
		Blueprint: soongCcLibraryPreamble + `
cc_library {
    name: "foo_both",
    srcs: ["foo.c"],
    include_build_directory: false,
    required: ["bar"],
}
cc_library_shared {
    name: "foo_shared",
    srcs: ["foo.c"],
    include_build_directory: false,
    required: ["bar"],
}
cc_library_static {
    name: "foo_static",
    srcs: ["foo.c"],
    include_build_directory: false,
    required: ["bar"],
}
cc_library_static {
    name: "bar",
    srcs: ["bar.c"],
    include_build_directory: false,
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "foo_both_bp2build_cc_library_static", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo_both", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_shared", "foo_shared", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_static", "foo_static", AttrNameToString{
				"srcs_c": `["foo.c"]`,
			}),
			MakeBazelTarget("cc_library_static", "bar", AttrNameToString{
				"srcs_c": `["bar.c"]`,
			}),
		},
	})
}

func TestPropertiesIfStubLibraryIsInNdk(t *testing.T) {
	tc := Bp2buildTestCase{
		Description:                "If an equivalent ndk_library exists, set included_in_ndk=true for module-libapi stubs",
		ModuleTypeUnderTest:        "cc_library",
		ModuleTypeUnderTestFactory: cc.LibraryFactory,
		Blueprint: `
// libfoo is an ndk library and contributes to module-libapi
cc_library {
	name: "libfoo",
	stubs: {symbol_file: "libfoo.map.txt"},
}
ndk_library {
	name: "libfoo",
	first_version: "29",
	symbol_file: "libfoo.map.txt",
}
// libbar is not an ndk library, but contributes to module-libapi
cc_library {
	name: "libbar",
	stubs: {symbol_file: "libbar.map.txt"},
}
`,
		StubbedBuildDefinitions: []string{"libfoo.ndk"},
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_library_static", "libfoo_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "libfoo", AttrNameToString{
				"local_includes":    `["."]`,
				"stubs_symbol_file": `"libfoo.map.txt"`,
			}),
			MakeBazelTarget("cc_stub_suite", "libfoo_stub_libs", AttrNameToString{
				"api_surface":          `"module-libapi"`,
				"soname":               `"libfoo.so"`,
				"source_library_label": `"//:libfoo"`,
				"symbol_file":          `"libfoo.map.txt"`,
				"versions":             `["current"]`,
				"included_in_ndk":      `True`,
			}),
			MakeBazelTarget("cc_library_static", "libbar_bp2build_cc_library_static", AttrNameToString{
				"local_includes": `["."]`,
			}),
			MakeBazelTarget("cc_library_shared", "libbar", AttrNameToString{
				"local_includes":    `["."]`,
				"stubs_symbol_file": `"libbar.map.txt"`,
			}),
			MakeBazelTarget("cc_stub_suite", "libbar_stub_libs", AttrNameToString{
				"api_surface":          `"module-libapi"`,
				"soname":               `"libbar.so"`,
				"source_library_label": `"//:libbar"`,
				"symbol_file":          `"libbar.map.txt"`,
				"versions":             `["current"]`,
			}),
		},
	}
	runCcLibraryTestCase(t, tc)
}
