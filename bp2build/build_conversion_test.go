// Copyright 2020 Google Inc. All rights reserved.
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
	"strings"
	"testing"

	"android/soong/android"
	"android/soong/android/allowlists"
	"android/soong/bazel"
	"android/soong/python"
)

func TestGenerateSoongModuleTargets(t *testing.T) {
	testCases := []struct {
		description         string
		bp                  string
		expectedBazelTarget string
	}{
		{
			description: "only name",
			bp: `custom { name: "foo" }
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
    string_prop = "",
)`,
		},
		{
			description: "handles bool",
			bp: `custom {
  name: "foo",
  bool_prop: true,
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = True,
    string_prop = "",
)`,
		},
		{
			description: "string escaping",
			bp: `custom {
  name: "foo",
  owner: "a_string_with\"quotes\"_and_\\backslashes\\\\",
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
    owner = "a_string_with\"quotes\"_and_\\backslashes\\\\",
    string_prop = "",
)`,
		},
		{
			description: "single item string list",
			bp: `custom {
  name: "foo",
  required: ["bar"],
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
    required = ["bar"],
    string_prop = "",
)`,
		},
		{
			description: "list of strings",
			bp: `custom {
  name: "foo",
  target_required: ["qux", "bazqux"],
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
    string_prop = "",
    target_required = [
        "qux",
        "bazqux",
    ],
)`,
		},
		{
			description: "dist/dists",
			bp: `custom {
  name: "foo",
  dist: {
    targets: ["goal_foo"],
    tag: ".foo",
  },
  dists: [{
    targets: ["goal_bar"],
    tag: ".bar",
  }],
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
    dist = {
        "tag": ".foo",
        "targets": ["goal_foo"],
    },
    dists = [{
        "tag": ".bar",
        "targets": ["goal_bar"],
    }],
    string_prop = "",
)`,
		},
		{
			description: "put it together",
			bp: `custom {
  name: "foo",
  required: ["bar"],
  target_required: ["qux", "bazqux"],
  bool_prop: true,
  owner: "custom_owner",
  dists: [
    {
      tag: ".tag",
      targets: ["my_goal"],
    },
  ],
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = True,
    dists = [{
        "tag": ".tag",
        "targets": ["my_goal"],
    }],
    owner = "custom_owner",
    required = ["bar"],
    string_prop = "",
    target_required = [
        "qux",
        "bazqux",
    ],
)`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := android.TestConfig(buildDir, nil, testCase.bp, nil)
			ctx := android.NewTestContext(config)

			ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
			ctx.Register()

			_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
			android.FailIfErrored(t, errs)
			_, errs = ctx.PrepareBuildActions(config)
			android.FailIfErrored(t, errs)

			codegenCtx := NewCodegenContext(config, ctx.Context, QueryView, "")
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.FailIfErrored(t, err)
			if actualCount, expectedCount := len(bazelTargets), 1; actualCount != expectedCount {
				t.Fatalf("Expected %d bazel target, got %d", expectedCount, actualCount)
			}

			actualBazelTarget := bazelTargets[0]
			if actualBazelTarget.content != testCase.expectedBazelTarget {
				t.Errorf(
					"Expected generated Bazel target to be '%s', got '%s'",
					testCase.expectedBazelTarget,
					actualBazelTarget.content,
				)
			}
		})
	}
}

func TestGenerateBazelTargetModules(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description: "string prop empty",
			Blueprint: `custom {
	name: "foo",
    string_literal_prop: "",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "foo", AttrNameToString{
					"string_literal_prop": `""`,
				}),
			},
		},
		{
			Description: `string prop "PROP"`,
			Blueprint: `custom {
	name: "foo",
    string_literal_prop: "PROP",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "foo", AttrNameToString{
					"string_literal_prop": `"PROP"`,
				}),
			},
		},
		{
			Description: `string prop arch variant`,
			Blueprint: `custom {
    name: "foo",
    arch: {
        arm: { string_literal_prop: "ARM" },
        arm64: { string_literal_prop: "ARM64" },
    },
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "foo", AttrNameToString{
					"string_literal_prop": `select({
        "//build/bazel_common_rules/platforms/arch:arm": "ARM",
        "//build/bazel_common_rules/platforms/arch:arm64": "ARM64",
        "//conditions:default": None,
    })`,
				}),
			},
		},
		{
			Description: "string ptr props",
			Blueprint: `custom {
	name: "foo",
    string_ptr_prop: "",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "foo", AttrNameToString{
					"string_ptr_prop": `""`,
				}),
			},
		},
		{
			Description: "string list props",
			Blueprint: `custom {
  name: "foo",
    string_list_prop: ["a", "b"],
    string_ptr_prop: "a",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "foo", AttrNameToString{
					"string_list_prop": `[
        "a",
        "b",
    ]`,
					"string_ptr_prop": `"a"`,
				}),
			},
		},
		{
			Description: "control characters",
			Blueprint: `custom {
    name: "foo",
    string_list_prop: ["\t", "\n"],
    string_ptr_prop: "a\t\n\r",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "foo", AttrNameToString{
					"string_list_prop": `[
        "\t",
        "\n",
    ]`,
					"string_ptr_prop": `"a\t\n\r"`,
				}),
			},
		},
		{
			Description: "handles dep",
			Blueprint: `custom {
  name: "has_dep",
  arch_paths: [":dep"],
  bazel_module: { bp2build_available: true },
}

custom {
  name: "dep",
  arch_paths: ["abc"],
  bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "dep", AttrNameToString{
					"arch_paths": `["abc"]`,
				}),
				MakeBazelTarget("custom", "has_dep", AttrNameToString{
					"arch_paths": `[":dep"]`,
				}),
			},
		},
		{
			Description: "arch-variant srcs",
			Blueprint: `custom {
    name: "arch_paths",
    arch: {
      x86: { arch_paths: ["x86.txt"] },
      x86_64:  { arch_paths: ["x86_64.txt"] },
      arm:  { arch_paths: ["arm.txt"] },
      arm64:  { arch_paths: ["arm64.txt"] },
      riscv64: { arch_paths: ["riscv64.txt"] },
    },
    target: {
      linux: { arch_paths: ["linux.txt"] },
      bionic: { arch_paths: ["bionic.txt"] },
      host: { arch_paths: ["host.txt"] },
      not_windows: { arch_paths: ["not_windows.txt"] },
      android: { arch_paths: ["android.txt"] },
      linux_musl: { arch_paths: ["linux_musl.txt"] },
      musl: { arch_paths: ["musl.txt"] },
      linux_glibc: { arch_paths: ["linux_glibc.txt"] },
      glibc: { arch_paths: ["glibc.txt"] },
      linux_bionic: { arch_paths: ["linux_bionic.txt"] },
      darwin: { arch_paths: ["darwin.txt"] },
      windows: { arch_paths: ["windows.txt"] },
    },
    multilib: {
        lib32: { arch_paths: ["lib32.txt"] },
        lib64: { arch_paths: ["lib64.txt"] },
    },
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "arch_paths", AttrNameToString{
					"arch_paths": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [
            "arm.txt",
            "lib32.txt",
        ],
        "//build/bazel_common_rules/platforms/arch:arm64": [
            "arm64.txt",
            "lib64.txt",
        ],
        "//build/bazel_common_rules/platforms/arch:riscv64": [
            "riscv64.txt",
            "lib64.txt",
        ],
        "//build/bazel_common_rules/platforms/arch:x86": [
            "x86.txt",
            "lib32.txt",
        ],
        "//build/bazel_common_rules/platforms/arch:x86_64": [
            "x86_64.txt",
            "lib64.txt",
        ],
        "//conditions:default": [],
    }) + select({
        "//build/bazel_common_rules/platforms/os:android": [
            "linux.txt",
            "bionic.txt",
            "android.txt",
        ],
        "//build/bazel_common_rules/platforms/os:darwin": [
            "host.txt",
            "darwin.txt",
            "not_windows.txt",
        ],
        "//build/bazel_common_rules/platforms/os:linux_bionic": [
            "host.txt",
            "linux.txt",
            "bionic.txt",
            "linux_bionic.txt",
            "not_windows.txt",
        ],
        "//build/bazel_common_rules/platforms/os:linux_glibc": [
            "host.txt",
            "linux.txt",
            "glibc.txt",
            "linux_glibc.txt",
            "not_windows.txt",
        ],
        "//build/bazel_common_rules/platforms/os:linux_musl": [
            "host.txt",
            "linux.txt",
            "musl.txt",
            "linux_musl.txt",
            "not_windows.txt",
        ],
        "//build/bazel_common_rules/platforms/os:windows": [
            "host.txt",
            "windows.txt",
        ],
        "//conditions:default": [],
    })`,
				}),
			},
		},
		{
			Description: "arch-variant deps",
			Blueprint: `custom {
  name: "has_dep",
  arch: {
    x86: {
      arch_paths: [":dep"],
    },
  },
  bazel_module: { bp2build_available: true },
}

custom {
    name: "dep",
    arch_paths: ["abc"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "dep", AttrNameToString{
					"arch_paths": `["abc"]`,
				}),
				MakeBazelTarget("custom", "has_dep", AttrNameToString{
					"arch_paths": `select({
        "//build/bazel_common_rules/platforms/arch:x86": [":dep"],
        "//conditions:default": [],
    })`,
				}),
			},
		},
		{
			Description: "embedded props",
			Blueprint: `custom {
    name: "embedded_props",
    embedded_prop: "abc",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "embedded_props", AttrNameToString{
					"embedded_attr": `"abc"`,
				}),
			},
		},
		{
			Description: "ptr to embedded props",
			Blueprint: `custom {
    name: "ptr_to_embedded_props",
    other_embedded_prop: "abc",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("custom", "ptr_to_embedded_props", AttrNameToString{
					"other_embedded_attr": `"abc"`,
				}),
			},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			config := android.TestConfig(buildDir, nil, testCase.Blueprint, nil)
			ctx := android.NewTestContext(config)

			registerCustomModuleForBp2buildConversion(ctx)

			_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
			if errored(t, testCase, errs) {
				return
			}
			_, errs = ctx.ResolveDependencies(config)
			if errored(t, testCase, errs) {
				return
			}

			codegenCtx := NewCodegenContext(config, ctx.Context, Bp2Build, "")
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.FailIfErrored(t, err)

			if actualCount, expectedCount := len(bazelTargets), len(testCase.ExpectedBazelTargets); actualCount != expectedCount {
				t.Errorf("Expected %d bazel target (%s),\ngot %d (%s)", expectedCount, testCase.ExpectedBazelTargets, actualCount, bazelTargets)
			} else {
				for i, expectedBazelTarget := range testCase.ExpectedBazelTargets {
					actualBazelTarget := bazelTargets[i]
					if actualBazelTarget.content != expectedBazelTarget {
						t.Errorf(
							"Expected generated Bazel target to be '%s', got '%s'",
							expectedBazelTarget,
							actualBazelTarget.content,
						)
					}
				}
			}
		})
	}
}

func TestBp2buildHostAndDevice(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description:                "host and device, device only",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.DeviceSupported),
			},
		},
		{
			Description:                "host and device, both",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: true,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{}),
			},
		},
		{
			Description:                "host and device, host explicitly disabled",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: false,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.DeviceSupported),
			},
		},
		{
			Description:                "host and device, neither",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: false,
		device_supported: false,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{
					"target_compatible_with": `["@platforms//:incompatible"]`,
				}),
			},
		},
		{
			Description:                "host and device, neither, cannot override with product_var",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: false,
		device_supported: false,
		product_variables: { unbundled_build: { enabled: true } },
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{
					"target_compatible_with": `["@platforms//:incompatible"]`,
				}),
			},
		},
		{
			Description:                "host and device, both, disabled overrided with product_var",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: true,
		device_supported: true,
		enabled: false,
		product_variables: { unbundled_build: { enabled: true } },
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{
					"target_compatible_with": `select({
        "//build/bazel/product_config/config_settings:unbundled_build": [],
        "//conditions:default": ["@platforms//:incompatible"],
    })`,
				}),
			},
		},
		{
			Description:                "host and device, neither, cannot override with arch enabled",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: false,
		device_supported: false,
		arch: { x86: { enabled: true } },
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{
					"target_compatible_with": `["@platforms//:incompatible"]`,
				}),
			},
		},
		{
			Description:                "host and device, host only",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			Blueprint: `custom {
		name: "foo",
		host_supported: true,
		device_supported: false,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.HostSupported),
			},
		},
		{
			Description:                "host only",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostSupported,
			Blueprint: `custom {
		name: "foo",
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.HostSupported),
			},
		},
		{
			Description:                "device only",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryDeviceSupported,
			Blueprint: `custom {
		name: "foo",
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.DeviceSupported),
			},
		},
		{
			Description:                "host and device default, default",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDeviceDefault,
			Blueprint: `custom {
		name: "foo",
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{}),
			},
		},
		{
			Description:                "host and device default, device only",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDeviceDefault,
			Blueprint: `custom {
		name: "foo",
		host_supported: false,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.DeviceSupported),
			},
		},
		{
			Description:                "host and device default, host only",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDeviceDefault,
			Blueprint: `custom {
		name: "foo",
		device_supported: false,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				makeBazelTargetHostOrDevice("custom", "foo", AttrNameToString{}, android.HostSupported),
			},
		},
		{
			Description:                "host and device default, neither",
			ModuleTypeUnderTest:        "custom",
			ModuleTypeUnderTestFactory: customModuleFactoryHostAndDeviceDefault,
			Blueprint: `custom {
		name: "foo",
		host_supported: false,
		device_supported: false,
		bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("custom", "foo", AttrNameToString{
					"target_compatible_with": `["@platforms//:incompatible"]`,
				}),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			RunBp2BuildTestCaseSimple(t, tc)
		})
	}
}

func TestLoadStatements(t *testing.T) {
	testCases := []struct {
		bazelTargets           BazelTargets
		expectedLoadStatements string
	}{
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:      "foo",
					ruleClass: "cc_library",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_library"}},
					}},
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_library")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:      "foo",
					ruleClass: "cc_library",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_library"}},
					}},
				},
				BazelTarget{
					name:      "bar",
					ruleClass: "cc_library",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_library"}},
					}},
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_library")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:      "foo",
					ruleClass: "cc_library",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_library"}},
					}},
				},
				BazelTarget{
					name:      "bar",
					ruleClass: "cc_binary",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_binary"}},
					}},
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_binary", "cc_library")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:      "foo",
					ruleClass: "cc_library",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_library"}},
					}},
				},
				BazelTarget{
					name:      "bar",
					ruleClass: "cc_binary",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_binary"}},
					}},
				},
				BazelTarget{
					name:      "baz",
					ruleClass: "java_binary",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:java.bzl",
						symbols: []BazelLoadSymbol{{symbol: "java_binary"}},
					}},
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_binary", "cc_library")
load("//build/bazel/rules:java.bzl", "java_binary")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:      "foo",
					ruleClass: "cc_binary",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:cc.bzl",
						symbols: []BazelLoadSymbol{{symbol: "cc_binary"}},
					}},
				},
				BazelTarget{
					name:      "bar",
					ruleClass: "java_binary",
					loads: []BazelLoad{{
						file:    "//build/bazel/rules:java.bzl",
						symbols: []BazelLoadSymbol{{symbol: "java_binary"}},
					}},
				},
				BazelTarget{
					name:      "baz",
					ruleClass: "genrule",
					// Note: no loads for native rules
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_binary")
load("//build/bazel/rules:java.bzl", "java_binary")`,
		},
	}

	for _, testCase := range testCases {
		actual := testCase.bazelTargets.LoadStatements()
		expected := testCase.expectedLoadStatements
		if actual != expected {
			t.Fatalf("Expected load statements to be %s, got %s", expected, actual)
		}
	}

}

func TestGenerateBazelTargetModules_OneToMany_LoadedFromStarlark(t *testing.T) {
	testCases := []struct {
		bp                       string
		expectedBazelTarget      string
		expectedBazelTargetCount int
		expectedLoadStatements   string
	}{
		{
			bp: `custom {
    name: "bar",
    host_supported: true,
    one_to_many_prop: true,
    bazel_module: { bp2build_available: true  },
}`,
			expectedBazelTarget: `my_library(
    name = "bar",
)

proto_library(
    name = "bar_proto_library_deps",
)

my_proto_library(
    name = "bar_my_proto_library_deps",
)`,
			expectedBazelTargetCount: 3,
			expectedLoadStatements: `load("//build/bazel/rules:proto.bzl", "my_proto_library", "proto_library")
load("//build/bazel/rules:rules.bzl", "my_library")`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		config := android.TestConfig(buildDir, nil, testCase.bp, nil)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		codegenCtx := NewCodegenContext(config, ctx.Context, Bp2Build, "")
		bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
		android.FailIfErrored(t, err)
		if actualCount := len(bazelTargets); actualCount != testCase.expectedBazelTargetCount {
			t.Fatalf("Expected %d bazel target, got %d", testCase.expectedBazelTargetCount, actualCount)
		}

		actualBazelTargets := bazelTargets.String()
		if actualBazelTargets != testCase.expectedBazelTarget {
			t.Errorf(
				"Expected generated Bazel target to be '%s', got '%s'",
				testCase.expectedBazelTarget,
				actualBazelTargets,
			)
		}

		actualLoadStatements := bazelTargets.LoadStatements()
		if actualLoadStatements != testCase.expectedLoadStatements {
			t.Errorf(
				"Expected generated load statements to be '%s', got '%s'",
				testCase.expectedLoadStatements,
				actualLoadStatements,
			)
		}
	}
}

func TestModuleTypeBp2Build(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description:                "filegroup with does not specify srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{}),
			},
		},
		{
			Description:                "filegroup with no srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: [],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{}),
			},
		},
		{
			Description:                "filegroup with srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a",
        "b",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with dot-slash-prefixed srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["./a", "./b"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a",
        "b",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with excludes srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["a", "b"],
    exclude_srcs: ["a"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `["b"]`,
				}),
			},
		},
		{
			Description:                "depends_on_other_dir_module",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: [
        ":foo",
        "c",
    ],
    bazel_module: { bp2build_available: true },
}`,
			Filesystem: map[string]string{
				"other/Android.bp": `filegroup {
    name: "foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: true },
}`,
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "//other:foo",
        "c",
    ]`,
				}),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			RunBp2BuildTestCase(t, func(ctx android.RegistrationContext) {}, testCase)
		})
	}
}

func TestAllowlistingBp2buildTargetsExplicitly(t *testing.T) {
	testCases := []struct {
		moduleTypeUnderTest        string
		moduleTypeUnderTestFactory android.ModuleFactory
		bp                         string
		expectedCount              int
		description                string
	}{
		{
			description:                "explicitly unavailable",
			moduleTypeUnderTest:        "filegroup",
			moduleTypeUnderTestFactory: android.FileGroupFactory,
			bp: `filegroup {
    name: "foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: false },
}`,
			expectedCount: 0,
		},
		{
			description:                "implicitly unavailable",
			moduleTypeUnderTest:        "filegroup",
			moduleTypeUnderTestFactory: android.FileGroupFactory,
			bp: `filegroup {
    name: "foo",
    srcs: ["a", "b"],
}`,
			expectedCount: 0,
		},
		{
			description:                "explicitly available",
			moduleTypeUnderTest:        "filegroup",
			moduleTypeUnderTestFactory: android.FileGroupFactory,
			bp: `filegroup {
    name: "foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: true },
}`,
			expectedCount: 1,
		},
		{
			description:                "generates more than 1 target if needed",
			moduleTypeUnderTest:        "custom",
			moduleTypeUnderTestFactory: customModuleFactoryHostAndDevice,
			bp: `custom {
    name: "foo",
    one_to_many_prop: true,
    bazel_module: { bp2build_available: true },
}`,
			expectedCount: 3,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := android.TestConfig(buildDir, nil, testCase.bp, nil)
			ctx := android.NewTestContext(config)
			ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
			ctx.RegisterForBazelConversion()

			_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
			android.FailIfErrored(t, errs)
			_, errs = ctx.ResolveDependencies(config)
			android.FailIfErrored(t, errs)

			codegenCtx := NewCodegenContext(config, ctx.Context, Bp2Build, "")
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.FailIfErrored(t, err)
			if actualCount := len(bazelTargets); actualCount != testCase.expectedCount {
				t.Fatalf("%s: Expected %d bazel target, got %d", testCase.description, testCase.expectedCount, actualCount)
			}
		})
	}
}

func TestAllowlistingBp2buildTargetsWithConfig(t *testing.T) {
	testCases := []struct {
		moduleTypeUnderTest        string
		moduleTypeUnderTestFactory android.ModuleFactory
		expectedCount              map[string]int
		description                string
		bp2buildConfig             allowlists.Bp2BuildConfig
		checkDir                   string
		fs                         map[string]string
		forceEnabledModules        []string
		expectedErrorMessages      []string
	}{
		{
			description:                "test bp2build config package and subpackages config",
			moduleTypeUnderTest:        "filegroup",
			moduleTypeUnderTestFactory: android.FileGroupFactory,
			expectedCount: map[string]int{
				"migrated":                           1,
				"migrated/but_not_really":            0,
				"migrated/but_not_really/but_really": 1,
				"not_migrated":                       0,
				"also_not_migrated":                  0,
			},
			bp2buildConfig: allowlists.Bp2BuildConfig{
				"migrated":                allowlists.Bp2BuildDefaultTrueRecursively,
				"migrated/but_not_really": allowlists.Bp2BuildDefaultFalse,
				"not_migrated":            allowlists.Bp2BuildDefaultFalse,
			},
			fs: map[string]string{
				"migrated/Android.bp":                           `filegroup { name: "a" }`,
				"migrated/but_not_really/Android.bp":            `filegroup { name: "b" }`,
				"migrated/but_not_really/but_really/Android.bp": `filegroup { name: "c" }`,
				"not_migrated/Android.bp":                       `filegroup { name: "d" }`,
				"also_not_migrated/Android.bp":                  `filegroup { name: "e" }`,
			},
		},
		{
			description:                "test bp2build config opt-in and opt-out",
			moduleTypeUnderTest:        "filegroup",
			moduleTypeUnderTestFactory: android.FileGroupFactory,
			expectedCount: map[string]int{
				"package-opt-in":             2,
				"package-opt-in/subpackage":  0,
				"package-opt-out":            1,
				"package-opt-out/subpackage": 0,
			},
			bp2buildConfig: allowlists.Bp2BuildConfig{
				"package-opt-in":  allowlists.Bp2BuildDefaultFalse,
				"package-opt-out": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			fs: map[string]string{
				"package-opt-in/Android.bp": `
filegroup { name: "opt-in-a" }
filegroup { name: "opt-in-b", bazel_module: { bp2build_available: true } }
filegroup { name: "opt-in-c", bazel_module: { bp2build_available: true } }
`,

				"package-opt-in/subpackage/Android.bp": `
filegroup { name: "opt-in-d" } // parent package not configured to DefaultTrueRecursively
`,

				"package-opt-out/Android.bp": `
filegroup { name: "opt-out-a" }
filegroup { name: "opt-out-b", bazel_module: { bp2build_available: false } }
filegroup { name: "opt-out-c", bazel_module: { bp2build_available: false } }
`,

				"package-opt-out/subpackage/Android.bp": `
filegroup { name: "opt-out-g", bazel_module: { bp2build_available: false } }
filegroup { name: "opt-out-h", bazel_module: { bp2build_available: false } }
`,
			},
		},
		{
			description:                "test force-enabled errors out",
			moduleTypeUnderTest:        "filegroup",
			moduleTypeUnderTestFactory: android.FileGroupFactory,
			expectedCount: map[string]int{
				"migrated":     0,
				"not_migrated": 0,
			},
			bp2buildConfig: allowlists.Bp2BuildConfig{
				"migrated/but_not_really": allowlists.Bp2BuildDefaultFalse,
				"not_migrated":            allowlists.Bp2BuildDefaultFalse,
			},
			fs: map[string]string{
				"migrated/Android.bp": `filegroup { name: "a" }`,
			},
			forceEnabledModules:   []string{"a"},
			expectedErrorMessages: []string{"Force Enabled Module a not converted"},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		fs := make(map[string][]byte)
		toParse := []string{
			"Android.bp",
		}
		for f, content := range testCase.fs {
			if strings.HasSuffix(f, "Android.bp") {
				toParse = append(toParse, f)
			}
			fs[f] = []byte(content)
		}
		config := android.TestConfig(buildDir, nil, "", fs)
		config.AddForceEnabledModules(testCase.forceEnabledModules)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
		allowlist := android.NewBp2BuildAllowlist().SetDefaultConfig(testCase.bp2buildConfig)
		ctx.RegisterBp2BuildConfig(allowlist)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, toParse)
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		codegenCtx := NewCodegenContext(config, ctx.Context, Bp2Build, "")

		// For each directory, test that the expected number of generated targets is correct.
		for dir, expectedCount := range testCase.expectedCount {
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.CheckErrorsAgainstExpectations(t, err, testCase.expectedErrorMessages)
			if actualCount := len(bazelTargets); actualCount != expectedCount {
				t.Fatalf(
					"%s: Expected %d bazel target for %s package, got %d",
					testCase.description,
					expectedCount,
					dir,
					actualCount)
			}

		}
	}
}

func TestCombineBuildFilesBp2buildTargets(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description:                "filegroup bazel_module.label",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    bazel_module: { label: "//other:fg_foo" },
}`,
			ExpectedBazelTargets: []string{},
			Filesystem: map[string]string{
				"other/BUILD.bazel": `// BUILD file`,
			},
		},
		{
			Description:                "multiple bazel_module.label same BUILD",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
        name: "fg_foo",
        bazel_module: { label: "//other:fg_foo" },
    }

    filegroup {
        name: "foo",
        bazel_module: { label: "//other:foo" },
    }`,
			ExpectedBazelTargets: []string{},
			Filesystem: map[string]string{
				"other/BUILD.bazel": `// BUILD file`,
			},
		},
		{
			Description:                "filegroup bazel_module.label and bp2build in subdir",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Dir:                        "other",
			Blueprint:                  ``,
			Filesystem: map[string]string{
				"other/Android.bp": `filegroup {
        name: "fg_foo",
        bazel_module: {
          bp2build_available: true,
        },
      }
      filegroup {
        name: "fg_bar",
        bazel_module: {
          label: "//other:fg_bar"
        },
      }`,
				"other/BUILD.bazel": `// definition for fg_bar`,
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{}),
			},
		},
		{
			Description:                "filegroup bazel_module.label and filegroup bp2build",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,

			Filesystem: map[string]string{
				"other/BUILD.bazel": `// BUILD file`,
			},
			Blueprint: `filegroup {
        name: "fg_foo",
        bazel_module: {
          label: "//other:fg_foo",
        },
    }

    filegroup {
        name: "fg_bar",
        bazel_module: {
          bp2build_available: true,
        },
    }`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_bar", map[string]string{}),
			},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			fs := make(map[string][]byte)
			toParse := []string{
				"Android.bp",
			}
			for f, content := range testCase.Filesystem {
				if strings.HasSuffix(f, "Android.bp") {
					toParse = append(toParse, f)
				}
				fs[f] = []byte(content)
			}
			config := android.TestConfig(buildDir, nil, testCase.Blueprint, fs)
			ctx := android.NewTestContext(config)
			ctx.RegisterModuleType(testCase.ModuleTypeUnderTest, testCase.ModuleTypeUnderTestFactory)
			ctx.RegisterForBazelConversion()

			_, errs := ctx.ParseFileList(dir, toParse)
			if errored(t, testCase, errs) {
				return
			}
			_, errs = ctx.ResolveDependencies(config)
			if errored(t, testCase, errs) {
				return
			}

			checkDir := dir
			if testCase.Dir != "" {
				checkDir = testCase.Dir
			}
			codegenCtx := NewCodegenContext(config, ctx.Context, Bp2Build, "")
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, checkDir)
			android.FailIfErrored(t, err)
			bazelTargets.sort()
			actualCount := len(bazelTargets)
			expectedCount := len(testCase.ExpectedBazelTargets)
			if actualCount != expectedCount {
				t.Errorf("Expected %d bazel target, got %d\n%s", expectedCount, actualCount, bazelTargets)
			}
			for i, target := range bazelTargets {
				actualContent := target.content
				expectedContent := testCase.ExpectedBazelTargets[i]
				if expectedContent != actualContent {
					t.Errorf(
						"Expected generated Bazel target to be '%s', got '%s'",
						expectedContent,
						actualContent,
					)
				}
			}
		})
	}
}

func TestGlob(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description:                "filegroup with glob",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "other/a.txt",
        "other/b.txt",
        "other/subdir/a.txt",
    ]`,
				}),
			},
			Filesystem: map[string]string{
				"other/a.txt":        "",
				"other/b.txt":        "",
				"other/subdir/a.txt": "",
				"other/file":         "",
			},
		},
		{
			Description:                "filegroup with glob in subdir",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Dir:                        "other",
			Filesystem: map[string]string{
				"other/Android.bp": `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
				"other/a.txt":        "",
				"other/b.txt":        "",
				"other/subdir/a.txt": "",
				"other/file":         "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "subdir/a.txt",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with glob with no kept BUILD files",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			KeepBuildFileForDirs:       []string{
				// empty
			},
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
			Filesystem: map[string]string{
				"a.txt":         "",
				"b.txt":         "",
				"foo/BUILD":     "",
				"foo/a.txt":     "",
				"foo/bar/BUILD": "",
				"foo/bar/b.txt": "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "foo/a.txt",
        "foo/bar/b.txt",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with glob with kept BUILD file",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			KeepBuildFileForDirs: []string{
				"foo",
			},
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
			Filesystem: map[string]string{
				"a.txt":         "",
				"b.txt":         "",
				"foo/BUILD":     "",
				"foo/a.txt":     "",
				"foo/bar/BUILD": "",
				"foo/bar/b.txt": "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "//foo:a.txt",
        "//foo:bar/b.txt",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with glob with kept BUILD.bazel file",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			KeepBuildFileForDirs: []string{
				"foo",
			},
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
			Filesystem: map[string]string{
				"a.txt":               "",
				"b.txt":               "",
				"foo/BUILD.bazel":     "",
				"foo/a.txt":           "",
				"foo/bar/BUILD.bazel": "",
				"foo/bar/b.txt":       "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "//foo:a.txt",
        "//foo:bar/b.txt",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with glob with Android.bp file as boundary",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
			Filesystem: map[string]string{
				"a.txt":              "",
				"b.txt":              "",
				"foo/Android.bp":     "",
				"foo/a.txt":          "",
				"foo/bar/Android.bp": "",
				"foo/bar/b.txt":      "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "//foo:a.txt",
        "//foo/bar:b.txt",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup with glob in subdir with kept BUILD and BUILD.bazel file",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Dir:                        "other",
			KeepBuildFileForDirs: []string{
				"other/foo",
				"other/foo/bar",
				// deliberately not other/foo/baz/BUILD.
			},
			Filesystem: map[string]string{
				"other/Android.bp": `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
				"other/a.txt":               "",
				"other/b.txt":               "",
				"other/foo/BUILD":           "",
				"other/foo/a.txt":           "",
				"other/foo/bar/BUILD.bazel": "",
				"other/foo/bar/b.txt":       "",
				"other/foo/baz/BUILD":       "",
				"other/foo/baz/c.txt":       "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "//other/foo:a.txt",
        "//other/foo/bar:b.txt",
        "//other/foo:baz/c.txt",
    ]`,
				}),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			RunBp2BuildTestCaseSimple(t, testCase)
		})
	}
}

func TestGlobExcludeSrcs(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description:                "filegroup top level exclude_srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    exclude_srcs: ["c.txt"],
    bazel_module: { bp2build_available: true },
}`,
			Filesystem: map[string]string{
				"a.txt":          "",
				"b.txt":          "",
				"c.txt":          "",
				"dir/Android.bp": "",
				"dir/e.txt":      "",
				"dir/f.txt":      "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "b.txt",
        "//dir:e.txt",
        "//dir:f.txt",
    ]`,
				}),
			},
		},
		{
			Description:                "filegroup in subdir exclude_srcs",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint:                  "",
			Dir:                        "dir",
			Filesystem: map[string]string{
				"dir/Android.bp": `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    exclude_srcs: ["b.txt"],
    bazel_module: { bp2build_available: true },
}
`,
				"dir/a.txt":             "",
				"dir/b.txt":             "",
				"dir/subdir/Android.bp": "",
				"dir/subdir/e.txt":      "",
				"dir/subdir/f.txt":      "",
			},
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"srcs": `[
        "a.txt",
        "//dir/subdir:e.txt",
        "//dir/subdir:f.txt",
    ]`,
				}),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			RunBp2BuildTestCaseSimple(t, testCase)
		})
	}
}

func TestCommonBp2BuildModuleAttrs(t *testing.T) {
	testCases := []Bp2buildTestCase{
		{
			Description:                "Required into data test",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			StubbedBuildDefinitions:    []string{"reqd"},
			Blueprint: simpleModule("filegroup", "reqd") + `
filegroup {
    name: "fg_foo",
    required: ["reqd"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"data": `[":reqd"]`,
				}),
			},
		},
		{
			Description:                "Required into data test, cyclic self reference is filtered out",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			StubbedBuildDefinitions:    []string{"reqd"},
			Blueprint: simpleModule("filegroup", "reqd") + `
filegroup {
    name: "fg_foo",
    required: ["reqd", "fg_foo"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"data": `[":reqd"]`,
				}),
			},
		},
		{
			Description:                "Required via arch into data test",
			ModuleTypeUnderTest:        "python_library",
			ModuleTypeUnderTestFactory: python.PythonLibraryFactory,
			StubbedBuildDefinitions:    []string{"reqdx86", "reqdarm"},
			Blueprint: simpleModule("python_library", "reqdx86") +
				simpleModule("python_library", "reqdarm") + `
python_library {
    name: "fg_foo",
    arch: {
       arm: {
         required: ["reqdarm"],
       },
       x86: {
         required: ["reqdx86"],
       },
    },
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("py_library", "fg_foo", map[string]string{
					"data": `select({
        "//build/bazel_common_rules/platforms/arch:arm": [":reqdarm"],
        "//build/bazel_common_rules/platforms/arch:x86": [":reqdx86"],
        "//conditions:default": [],
    })`,
					"srcs_version": `"PY3"`,
					"imports":      `["."]`,
				}),
			},
		},
		{
			Description:                "Required appended to data test",
			ModuleTypeUnderTest:        "python_library",
			ModuleTypeUnderTestFactory: python.PythonLibraryFactory,
			Filesystem: map[string]string{
				"data.bin": "",
				"src.py":   "",
			},
			StubbedBuildDefinitions: []string{"reqd"},
			Blueprint: simpleModule("python_library", "reqd") + `
python_library {
    name: "fg_foo",
    data: ["data.bin"],
    required: ["reqd"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTarget("py_library", "fg_foo", map[string]string{
					"data": `[
        "data.bin",
        ":reqd",
    ]`,
					"srcs_version": `"PY3"`,
					"imports":      `["."]`,
				}),
			},
		},
		{
			Description:                "All props-to-attrs at once together test",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			StubbedBuildDefinitions:    []string{"reqd"},
			Blueprint: simpleModule("filegroup", "reqd") + `
filegroup {
    name: "fg_foo",
    required: ["reqd"],
    bazel_module: { bp2build_available: true },
}`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "fg_foo", map[string]string{
					"data": `[":reqd"]`,
				}),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			RunBp2BuildTestCaseSimple(t, tc)
		})
	}
}

func TestLicensesAttrConversion(t *testing.T) {
	RunBp2BuildTestCase(t,
		func(ctx android.RegistrationContext) {
			ctx.RegisterModuleType("license", android.LicenseFactory)
		},
		Bp2buildTestCase{
			Description:                "Test that licenses: attribute is converted",
			ModuleTypeUnderTest:        "filegroup",
			ModuleTypeUnderTestFactory: android.FileGroupFactory,
			Blueprint: `
license {
    name: "my_license",
}
filegroup {
    name: "my_filegroup",
    licenses: ["my_license"],
}
`,
			ExpectedBazelTargets: []string{
				MakeBazelTargetNoRestrictions("filegroup", "my_filegroup", AttrNameToString{
					"applicable_licenses": `[":my_license"]`,
				}),
				MakeBazelTargetNoRestrictions("android_license", "my_license", AttrNameToString{}),
			},
		})
}

func TestGenerateConfigSetting(t *testing.T) {
	bp := `
	custom {
		name: "foo",
		test_config_setting: true,
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTargetNoRestrictions(
			"config_setting",
			"foo_config_setting",
			AttrNameToString{
				"flag_values": `{
        "//build/bazel/rules/my_string_setting": "foo",
    }`,
			},
		),
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Blueprint:            bp,
		ExpectedBazelTargets: expectedBazelTargets,
		Description:          "Generating API contribution Bazel targets for custom module",
	})
}

// If values of all keys in an axis are equal to //conditions:default, drop the axis and print the common value
func TestPrettyPrintSelectMapEqualValues(t *testing.T) {
	lla := bazel.LabelListAttribute{
		Value: bazel.LabelList{},
	}
	libFooImplLabel := bazel.Label{
		Label: ":libfoo.impl",
	}
	lla.SetSelectValue(bazel.OsAndInApexAxis, bazel.AndroidPlatform, bazel.MakeLabelList([]bazel.Label{libFooImplLabel}))
	lla.SetSelectValue(bazel.OsAndInApexAxis, bazel.ConditionsDefaultConfigKey, bazel.MakeLabelList([]bazel.Label{libFooImplLabel}))
	actual, _ := prettyPrintAttribute(lla, 0)
	android.AssertStringEquals(t, "Print the common value if all keys in an axis have the same value", `[":libfoo.impl"]`, actual)
}

func TestAlreadyPresentBuildTarget(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	}
	custom {
		name: "bar",
	}
	`
	alreadyPresentBuildFile :=
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{},
		)
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"bar",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		AlreadyExistingBuildContents: alreadyPresentBuildFile,
		Blueprint:                    bp,
		ExpectedBazelTargets:         expectedBazelTargets,
		Description:                  "Not duplicating work for an already-present BUILD target.",
	})
}

func TestAlreadyPresentOneToManyBuildTarget(t *testing.T) {
	bp := `
	custom {
		name: "foo",
    one_to_many_prop: true,
	}
	custom {
		name: "bar",
	}
	`
	alreadyPresentBuildFile :=
		MakeBazelTarget(
			"custom",
			// one_to_many_prop ensures that foo generates "foo_proto_library_deps".
			"foo_proto_library_deps",
			AttrNameToString{},
		)
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"bar",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		AlreadyExistingBuildContents: alreadyPresentBuildFile,
		Blueprint:                    bp,
		ExpectedBazelTargets:         expectedBazelTargets,
		Description:                  "Not duplicating work for an already-present BUILD target (different generated name)",
	})
}

// Verifies that if a module is defined in pkg1/Android.bp, that a target present
// in pkg2/BUILD.bazel does not result in the module being labeled "already defined
// in a BUILD file".
func TestBuildTargetPresentOtherDirectory(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		KeepBuildFileForDirs: []string{"other_pkg"},
		Filesystem: map[string]string{
			"other_pkg/BUILD.bazel": MakeBazelTarget("custom", "foo", AttrNameToString{}),
		},
		Blueprint:            bp,
		ExpectedBazelTargets: expectedBazelTargets,
		Description:          "Not treating a BUILD target as the bazel definition for a module in another package",
	})
}

// If CommonAttributes.Dir is set, the bazel target should be created in that dir
func TestCreateBazelTargetInDifferentDir(t *testing.T) {
	t.Parallel()
	bp := `
	custom {
		name: "foo",
		dir: "subdir",
	}
	`
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	// Check that foo is not created in root dir
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Description: "foo is not created in root dir because it sets dir explicitly",
		Blueprint:   bp,
		Filesystem: map[string]string{
			"subdir/Android.bp": "",
		},
		ExpectedBazelTargets: []string{},
	})
	// Check that foo is created in `subdir`
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Description: "foo is created in `subdir` because it sets dir explicitly",
		Blueprint:   bp,
		Filesystem: map[string]string{
			"subdir/Android.bp": "",
		},
		Dir: "subdir",
		ExpectedBazelTargets: []string{
			MakeBazelTarget("custom", "foo", AttrNameToString{}),
		},
	})
	// Check that we cannot create target in different dir if it is does not an Android.bp
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Description: "foo cannot be created in `subdir` because it does not contain an Android.bp file",
		Blueprint:   bp,
		Dir:         "subdir",
		ExpectedErr: fmt.Errorf("Cannot use ca.Dir to create a BazelTarget in dir: subdir since it does not contain an Android.bp file"),
	})

}

func TestBp2buildDepsMutator_missingTransitiveDep(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	}

	custom {
		name: "has_deps",
	  arch_paths: [":has_missing_dep", ":foo"],
	}

	custom {
		name: "has_missing_dep",
	  arch_paths: [":missing"],
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Blueprint:            bp,
		ExpectedBazelTargets: expectedBazelTargets,
		Description:          "Skipping conversion of a target with missing transitive dep",
	})
}

func TestBp2buildDepsMutator_missingDirectDep(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	  arch_paths: [":exists"],
	}
	custom {
		name: "exists",
	}

	custom {
		name: "has_missing_dep",
	  arch_paths: [":missing"],
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{"arch_paths": `[":exists"]`},
		),
		MakeBazelTarget(
			"custom",
			"exists",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Blueprint:            bp,
		ExpectedBazelTargets: expectedBazelTargets,
		Description:          "Skipping conversion of a target with missing direct dep",
	})
}

func TestBp2buildDepsMutator_unconvertedDirectDep(t *testing.T) {
	bp := `
	custom {
		name: "has_unconverted_dep",
	  arch_paths: [":unconvertible"],
	}

	custom {
		name: "unconvertible",
		does_not_convert_to_bazel: true
	}
	`
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Blueprint:            bp,
		ExpectedBazelTargets: []string{},
		Description:          "Skipping conversion of a target with unconverted direct dep",
	})
}

func TestBp2buildDepsMutator_unconvertedTransitiveDep(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	  arch_paths: [":has_unconverted_dep", ":bar"],
	}

	custom {
		name: "bar",
	}

	custom {
		name: "has_unconverted_dep",
	  arch_paths: [":unconvertible"],
	}

	custom {
		name: "unconvertible",
		does_not_convert_to_bazel: true
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"bar",
			AttrNameToString{},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		Blueprint:            bp,
		ExpectedBazelTargets: expectedBazelTargets,
		Description:          "Skipping conversion of a target with unconverted transitive dep",
	})
}

func TestBp2buildDepsMutator_alreadyExistsBuildDeps(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	  arch_paths: [":bar"],
	}
	custom {
		name: "bar",
	  arch_paths: [":checked_in"],
	}
	custom {
		name: "checked_in",
	  arch_paths: [":checked_in"],
		does_not_convert_to_bazel: true
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{"arch_paths": `[":bar"]`},
		),
		MakeBazelTarget(
			"custom",
			"bar",
			AttrNameToString{"arch_paths": `[":checked_in"]`},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		StubbedBuildDefinitions: []string{"checked_in"},
		Blueprint:               bp,
		ExpectedBazelTargets:    expectedBazelTargets,
		Description:             "Convert target with already-existing build dep",
	})
}

// Tests that deps of libc are always considered valid for libc. This circumvents
// an issue that, in a variantless graph (such as bp2build's), libc has the
// unique predicament that it depends on itself.
func TestBp2buildDepsMutator_depOnLibc(t *testing.T) {
	bp := `
	custom {
		name: "foo",
	  arch_paths: [":libc"],
	}
	custom {
		name: "libc",
	  arch_paths: [":libc_dep"],
	}
	custom {
		name: "libc_dep",
		does_not_convert_to_bazel: true
	}
	`
	expectedBazelTargets := []string{
		MakeBazelTarget(
			"custom",
			"foo",
			AttrNameToString{"arch_paths": `[":libc"]`},
		),
		MakeBazelTarget(
			"custom",
			"libc",
			AttrNameToString{"arch_paths": `[":libc_dep"]`},
		),
	}
	registerCustomModule := func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
	}
	RunBp2BuildTestCase(t, registerCustomModule, Bp2buildTestCase{
		StubbedBuildDefinitions: []string{"checked_in"},
		Blueprint:               bp,
		ExpectedBazelTargets:    expectedBazelTargets,
		Description:             "Convert target with dep on libc",
	})
}
