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

	"android/soong/android"
	"android/soong/cc"
)

func runSoongConfigModuleTypeTest(t *testing.T, tc Bp2buildTestCase) {
	t.Helper()
	RunBp2BuildTestCase(t, registerSoongConfigModuleTypes, tc)
}

func registerSoongConfigModuleTypes(ctx android.RegistrationContext) {
	cc.RegisterCCBuildComponents(ctx)

	android.RegisterSoongConfigModuleBuildComponents(ctx)

	ctx.RegisterModuleType("cc_library", cc.LibraryFactory)
	ctx.RegisterModuleType("custom", customModuleFactoryHostAndDevice)
}

func TestErrorInBpFileDoesNotPanic(t *testing.T) {
	bp := `
soong_config_module_type {
    name: "library_linking_strategy_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "ANDROID",
    variables: ["library_linking_strategy"],
    properties: [
        "shared_libs",
        "static_libs",
    ],
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		ExpectedErr:                fmt.Errorf(`unknown variable "library_linking_strategy" in module type "library_linking_strategy_cc_defaults`),
	})
}

func TestSoongConfigModuleType(t *testing.T) {
	bp := `
soong_config_module_type {
	name: "custom_cc_library_static",
	module_type: "cc_library_static",
	config_namespace: "acme",
	bool_variables: ["feature1"],
	properties: ["cflags"],
}

custom_cc_library_static {
	name: "foo",
	bazel_module: { bp2build_available: true },
	host_supported: true,
	soong_config_variables: {
		feature1: {
			conditions_default: {
				cflags: ["-DDEFAULT1"],
			},
			cflags: ["-DFEATURE1"],
		},
	},
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - soong_config_module_type is supported in bp2build",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "foo",
    copts = select({
        "//build/bazel/product_config/config_settings:acme__feature1": ["-DFEATURE1"],
        "//conditions:default": ["-DDEFAULT1"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleTypeImport(t *testing.T) {
	configBp := `
soong_config_module_type {
	name: "custom_cc_library_static",
	module_type: "cc_library_static",
	config_namespace: "acme",
	bool_variables: ["feature1"],
	properties: ["cflags"],
}
`
	bp := `
soong_config_module_type_import {
	from: "foo/bar/SoongConfig.bp",
	module_types: ["custom_cc_library_static"],
}

custom_cc_library_static {
	name: "foo",
	bazel_module: { bp2build_available: true },
	host_supported: true,
	soong_config_variables: {
		feature1: {
			conditions_default: {
				cflags: ["-DDEFAULT1"],
			},
			cflags: ["-DFEATURE1"],
		},
	},
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - soong_config_module_type_import is supported in bp2build",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Filesystem: map[string]string{
			"foo/bar/SoongConfig.bp": configBp,
		},
		Blueprint: bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "foo",
    copts = select({
        "//build/bazel/product_config/config_settings:acme__feature1": ["-DFEATURE1"],
        "//conditions:default": ["-DDEFAULT1"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_StringVar(t *testing.T) {
	bp := `
soong_config_string_variable {
	name: "board",
	values: ["soc_a", "soc_b", "soc_c"],
}

soong_config_module_type {
	name: "custom_cc_library_static",
	module_type: "cc_library_static",
	config_namespace: "acme",
	variables: ["board"],
	properties: ["cflags"],
}

custom_cc_library_static {
	name: "foo",
	bazel_module: { bp2build_available: true },
	host_supported: true,
	soong_config_variables: {
		board: {
			soc_a: {
				cflags: ["-DSOC_A"],
			},
			soc_b: {
				cflags: ["-DSOC_B"],
			},
			soc_c: {},
			conditions_default: {
				cflags: ["-DSOC_DEFAULT"]
			},
		},
	},
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for string vars",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "foo",
    copts = select({
        "//build/bazel/product_config/config_settings:acme__board__soc_a": ["-DSOC_A"],
        "//build/bazel/product_config/config_settings:acme__board__soc_b": ["-DSOC_B"],
        "//build/bazel/product_config/config_settings:acme__board__soc_c": [],
        "//conditions:default": ["-DSOC_DEFAULT"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_MultipleBoolVar_PartialUseNotPanic(t *testing.T) {
	bp := `
soong_config_bool_variable {
	name: "feature1",
}

soong_config_bool_variable {
	name: "feature2",
}

soong_config_module_type {
	name: "custom_cc_library_static",
	module_type: "cc_library_static",
	config_namespace: "acme",
	variables: ["feature1", "feature2",],
	properties: ["cflags"],
}

custom_cc_library_static {
	name: "foo",
	bazel_module: { bp2build_available: true },
	host_supported: true,
	soong_config_variables: {
		feature1: {
			conditions_default: {
				cflags: ["-DDEFAULT1"],
			},
			cflags: ["-DFEATURE1"],
		},
	},
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - used part of multiple bool variable do not panic",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "foo",
    copts = select({
        "//build/bazel/product_config/config_settings:acme__feature1": ["-DFEATURE1"],
        "//conditions:default": ["-DDEFAULT1"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_StringAndBoolVar(t *testing.T) {
	bp := `
soong_config_bool_variable {
	name: "feature1",
}

soong_config_bool_variable {
	name: "feature2",
}

soong_config_string_variable {
	name: "board",
	values: ["soc_a", "soc_b", "soc_c", "soc_d"],
}

soong_config_module_type {
	name: "custom_cc_library_static",
	module_type: "cc_library_static",
	config_namespace: "acme",
	variables: ["feature1", "feature2", "board"],
	properties: ["cflags"],
}

custom_cc_library_static {
	name: "foo",
	bazel_module: { bp2build_available: true },
	host_supported: true,
	soong_config_variables: {
		feature1: {
			conditions_default: {
				cflags: ["-DDEFAULT1"],
			},
			cflags: ["-DFEATURE1"],
		},
		feature2: {
			cflags: ["-DFEATURE2"],
			conditions_default: {
				cflags: ["-DDEFAULT2"],
			},
		},
		board: {
			soc_a: {
				cflags: ["-DSOC_A"],
			},
			soc_b: {
				cflags: ["-DSOC_B"],
			},
			soc_c: {},
			conditions_default: {
				cflags: ["-DSOC_DEFAULT"]
			},
		},
	},
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for multiple variable types",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "foo",
    copts = select({
        "//build/bazel/product_config/config_settings:acme__board__soc_a": ["-DSOC_A"],
        "//build/bazel/product_config/config_settings:acme__board__soc_b": ["-DSOC_B"],
        "//build/bazel/product_config/config_settings:acme__board__soc_c": [],
        "//conditions:default": ["-DSOC_DEFAULT"],
    }) + select({
        "//build/bazel/product_config/config_settings:acme__feature1": ["-DFEATURE1"],
        "//conditions:default": ["-DDEFAULT1"],
    }) + select({
        "//build/bazel/product_config/config_settings:acme__feature2": ["-DFEATURE2"],
        "//conditions:default": ["-DDEFAULT2"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_StringVar_LabelListDeps(t *testing.T) {
	bp := `
soong_config_string_variable {
	name: "board",
	values: ["soc_a", "soc_b", "soc_c", "soc_d"],
}

soong_config_module_type {
	name: "custom_cc_library_static",
	module_type: "cc_library_static",
	config_namespace: "acme",
	variables: ["board"],
	properties: ["cflags", "static_libs"],
}

custom_cc_library_static {
	name: "foo",
	bazel_module: { bp2build_available: true },
	host_supported: true,
	soong_config_variables: {
		board: {
			soc_a: {
				cflags: ["-DSOC_A"],
				static_libs: ["soc_a_dep"],
			},
			soc_b: {
				cflags: ["-DSOC_B"],
				static_libs: ["soc_b_dep"],
			},
			soc_c: {},
			conditions_default: {
				cflags: ["-DSOC_DEFAULT"],
				static_libs: ["soc_default_static_dep"],
			},
		},
	},
}`

	otherDeps := `
cc_library_static { name: "soc_a_dep"}
cc_library_static { name: "soc_b_dep"}
cc_library_static { name: "soc_default_static_dep"}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for label list attributes",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		Filesystem: map[string]string{
			"foo/bar/Android.bp": otherDeps,
		},
		StubbedBuildDefinitions: []string{"//foo/bar:soc_a_dep", "//foo/bar:soc_b_dep", "//foo/bar:soc_default_static_dep"},
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "foo",
    copts = select({
        "//build/bazel/product_config/config_settings:acme__board__soc_a": ["-DSOC_A"],
        "//build/bazel/product_config/config_settings:acme__board__soc_b": ["-DSOC_B"],
        "//build/bazel/product_config/config_settings:acme__board__soc_c": [],
        "//conditions:default": ["-DSOC_DEFAULT"],
    }),
    implementation_deps = select({
        "//build/bazel/product_config/config_settings:acme__board__soc_a": ["//foo/bar:soc_a_dep"],
        "//build/bazel/product_config/config_settings:acme__board__soc_b": ["//foo/bar:soc_b_dep"],
        "//build/bazel/product_config/config_settings:acme__board__soc_c": [],
        "//conditions:default": ["//foo/bar:soc_default_static_dep"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_Defaults_SingleNamespace(t *testing.T) {
	bp := `
soong_config_module_type {
	name: "vendor_foo_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "vendor_foo",
	bool_variables: ["feature"],
	properties: ["cflags", "cppflags"],
}

vendor_foo_cc_defaults {
	name: "foo_defaults_1",
	soong_config_variables: {
		feature: {
			cflags: ["-cflag_feature_1"],
			conditions_default: {
				cflags: ["-cflag_default_1"],
			},
		},
	},
}

vendor_foo_cc_defaults {
	name: "foo_defaults_2",
	defaults: ["foo_defaults_1"],
	soong_config_variables: {
		feature: {
			cflags: ["-cflag_feature_2"],
			conditions_default: {
				cflags: ["-cflag_default_2"],
			},
		},
	},
}

cc_library_static {
	name: "lib",
	defaults: ["foo_defaults_2"],
	bazel_module: { bp2build_available: true },
	host_supported: true,
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - defaults with a single namespace",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "lib",
    copts = select({
        "//build/bazel/product_config/config_settings:vendor_foo__feature": [
            "-cflag_feature_2",
            "-cflag_feature_1",
        ],
        "//conditions:default": [
            "-cflag_default_2",
            "-cflag_default_1",
        ],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_MultipleDefaults_SingleNamespace(t *testing.T) {
	bp := `
soong_config_module_type {
	name: "foo_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "acme",
	bool_variables: ["feature"],
	properties: ["cflags"],
}

soong_config_module_type {
	name: "bar_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "acme",
	bool_variables: ["feature"],
	properties: ["cflags", "asflags"],
}

foo_cc_defaults {
	name: "foo_defaults",
	soong_config_variables: {
		feature: {
			cflags: ["-cflag_foo"],
			conditions_default: {
				cflags: ["-cflag_default_foo"],
			},
		},
	},
}

bar_cc_defaults {
	name: "bar_defaults",
	srcs: ["file.S"],
	soong_config_variables: {
		feature: {
			cflags: ["-cflag_bar"],
			asflags: ["-asflag_bar"],
			conditions_default: {
				asflags: ["-asflag_default_bar"],
				cflags: ["-cflag_default_bar"],
			},
		},
	},
}

cc_library_static {
	name: "lib",
	defaults: ["foo_defaults", "bar_defaults"],
	bazel_module: { bp2build_available: true },
	host_supported: true,
}

cc_library_static {
	name: "lib2",
	defaults: ["bar_defaults", "foo_defaults"],
	bazel_module: { bp2build_available: true },
	host_supported: true,
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - multiple defaults with a single namespace",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "lib",
    asflags = select({
        "//build/bazel/product_config/config_settings:acme__feature": ["-asflag_bar"],
        "//conditions:default": ["-asflag_default_bar"],
    }),
    copts = select({
        "//build/bazel/product_config/config_settings:acme__feature": [
            "-cflag_foo",
            "-cflag_bar",
        ],
        "//conditions:default": [
            "-cflag_default_foo",
            "-cflag_default_bar",
        ],
    }),
    local_includes = ["."],
    srcs_as = ["file.S"],
)`,
			`cc_library_static(
    name = "lib2",
    asflags = select({
        "//build/bazel/product_config/config_settings:acme__feature": ["-asflag_bar"],
        "//conditions:default": ["-asflag_default_bar"],
    }),
    copts = select({
        "//build/bazel/product_config/config_settings:acme__feature": [
            "-cflag_bar",
            "-cflag_foo",
        ],
        "//conditions:default": [
            "-cflag_default_bar",
            "-cflag_default_foo",
        ],
    }),
    local_includes = ["."],
    srcs_as = ["file.S"],
)`}})
}

func TestSoongConfigModuleType_Defaults_MultipleNamespaces(t *testing.T) {
	bp := `
soong_config_module_type {
	name: "vendor_foo_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "vendor_foo",
	bool_variables: ["feature"],
	properties: ["cflags"],
}

soong_config_module_type {
	name: "vendor_bar_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "vendor_bar",
	bool_variables: ["feature"],
	properties: ["cflags"],
}

soong_config_module_type {
	name: "vendor_qux_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "vendor_qux",
	bool_variables: ["feature"],
	properties: ["cflags"],
}

vendor_foo_cc_defaults {
	name: "foo_defaults",
	soong_config_variables: {
		feature: {
			cflags: ["-DVENDOR_FOO_FEATURE"],
			conditions_default: {
				cflags: ["-DVENDOR_FOO_DEFAULT"],
			},
		},
	},
}

vendor_bar_cc_defaults {
	name: "bar_defaults",
	soong_config_variables: {
		feature: {
			cflags: ["-DVENDOR_BAR_FEATURE"],
			conditions_default: {
				cflags: ["-DVENDOR_BAR_DEFAULT"],
			},
		},
	},
}

vendor_qux_cc_defaults {
	name: "qux_defaults",
	defaults: ["bar_defaults"],
	soong_config_variables: {
		feature: {
			cflags: ["-DVENDOR_QUX_FEATURE"],
			conditions_default: {
				cflags: ["-DVENDOR_QUX_DEFAULT"],
			},
		},
	},
}

cc_library_static {
	name: "lib",
	defaults: ["foo_defaults", "qux_defaults"],
	bazel_module: { bp2build_available: true },
	host_supported: true,
}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - defaults with multiple namespaces",
		ModuleTypeUnderTest:        "cc_library_static",
		ModuleTypeUnderTestFactory: cc.LibraryStaticFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{`cc_library_static(
    name = "lib",
    copts = select({
        "//build/bazel/product_config/config_settings:vendor_bar__feature": ["-DVENDOR_BAR_FEATURE"],
        "//conditions:default": ["-DVENDOR_BAR_DEFAULT"],
    }) + select({
        "//build/bazel/product_config/config_settings:vendor_foo__feature": ["-DVENDOR_FOO_FEATURE"],
        "//conditions:default": ["-DVENDOR_FOO_DEFAULT"],
    }) + select({
        "//build/bazel/product_config/config_settings:vendor_qux__feature": ["-DVENDOR_QUX_FEATURE"],
        "//conditions:default": ["-DVENDOR_QUX_DEFAULT"],
    }),
    local_includes = ["."],
)`}})
}

func TestSoongConfigModuleType_Defaults_UseBaselineValueForStringProp(t *testing.T) {
	bp := `
soong_config_string_variable {
    name: "library_linking_strategy",
    values: [
        "prefer_static",
    ],
}

soong_config_module_type {
    name: "library_linking_strategy_custom",
    module_type: "custom",
    config_namespace: "ANDROID",
    variables: ["library_linking_strategy"],
    properties: [
        "string_literal_prop",
    ],
}

library_linking_strategy_custom {
    name: "foo",
    string_literal_prop: "29",
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {},
            conditions_default: {
              string_literal_prop: "30",
            },
        },
    },
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem:                 map[string]string{},
		ExpectedBazelTargets: []string{
			MakeBazelTarget("custom", "foo", AttrNameToString{
				"string_literal_prop": `select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": "29",
        "//conditions:default": "30",
    })`,
			}),
		},
	})
}

func TestSoongConfigModuleType_UnsetConditions(t *testing.T) {
	bp := `
soong_config_string_variable {
    name: "library_linking_strategy",
    values: [
        "prefer_static",
    ],
}

soong_config_module_type {
    name: "library_linking_strategy_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "ANDROID",
    variables: ["library_linking_strategy"],
    properties: [
        "shared_libs",
        "static_libs",
    ],
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_lib_a_defaults",
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {},
            conditions_default: {
                shared_libs: [
                    "lib_a",
                ],
            },
        },
    },
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_merged_defaults",
    defaults: ["library_linking_strategy_lib_a_defaults"],
    host_supported: true,
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {},
            conditions_default: {
                shared_libs: [
                    "lib_b",
                ],
            },
        },
    },
}

cc_binary {
    name: "library_linking_strategy_sample_binary",
    srcs: ["library_linking_strategy.cc"],
    defaults: ["library_linking_strategy_merged_defaults"],
    include_build_directory: false,
}`

	otherDeps := `
cc_library { name: "lib_a"}
cc_library { name: "lib_b"}
cc_library { name: "lib_default"}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		StubbedBuildDefinitions:    []string{"//foo/bar:lib_a", "//foo/bar:lib_b", "//foo/bar:lib_default"},
		Filesystem: map[string]string{
			"foo/bar/Android.bp": otherDeps,
		},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "library_linking_strategy_sample_binary",
    dynamic_deps = select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [],
        "//conditions:default": [
            "//foo/bar:lib_b",
            "//foo/bar:lib_a",
        ],
    }),
    srcs = ["library_linking_strategy.cc"],
)`}})
}

func TestSoongConfigModuleType_UnsetConditionsExcludeLibs(t *testing.T) {
	bp := `
soong_config_string_variable {
    name: "library_linking_strategy",
    values: [
        "prefer_static",
    ],
}

soong_config_module_type {
    name: "library_linking_strategy_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "ANDROID",
    variables: ["library_linking_strategy"],
    properties: ["shared_libs"],
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_lib_a_defaults",
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {},
            conditions_default: {
                shared_libs: [
                    "lib_a",
                ],
            },
        },
    },
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_merged_defaults",
    defaults: ["library_linking_strategy_lib_a_defaults"],
    host_supported: true,
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {},
            conditions_default: {
                shared_libs: [
                    "lib_b",
                    "lib_c",
                ],
            },
        },
    },
    exclude_shared_libs: ["lib_a"],
}

cc_binary {
    name: "library_linking_strategy_sample_binary",
    defaults: ["library_linking_strategy_merged_defaults"],
    include_build_directory: false,
}

cc_binary {
    name: "library_linking_strategy_sample_binary_with_excludes",
    defaults: ["library_linking_strategy_merged_defaults"],
    exclude_shared_libs: ["lib_c"],
    include_build_directory: false,
}`

	otherDeps := `
cc_library { name: "lib_a"}
cc_library { name: "lib_b"}
cc_library { name: "lib_c"}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		StubbedBuildDefinitions:    []string{"//foo/bar:lib_a", "//foo/bar:lib_b", "//foo/bar:lib_c"},
		Blueprint:                  bp,
		Filesystem: map[string]string{
			"foo/bar/Android.bp": otherDeps,
		},
		ExpectedBazelTargets: []string{
			MakeBazelTargetNoRestrictions("cc_binary", "library_linking_strategy_sample_binary", AttrNameToString{
				"dynamic_deps": `select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [],
        "//conditions:default": [
            "//foo/bar:lib_b",
            "//foo/bar:lib_c",
        ],
    })`,
			}),
			MakeBazelTargetNoRestrictions("cc_binary", "library_linking_strategy_sample_binary_with_excludes", AttrNameToString{
				"dynamic_deps": `select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [],
        "//conditions:default": ["//foo/bar:lib_b"],
    })`,
			}),
		}})
}

func TestSoongConfigModuleType_Defaults(t *testing.T) {
	bp := `
soong_config_string_variable {
    name: "library_linking_strategy",
    values: [
        "prefer_static",
    ],
}

soong_config_module_type {
    name: "library_linking_strategy_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "ANDROID",
    variables: ["library_linking_strategy"],
    properties: [
        "shared_libs",
        "static_libs",
    ],
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_lib_a_defaults",
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {
                static_libs: [
                    "lib_a",
                ],
            },
            conditions_default: {
                shared_libs: [
                    "lib_a",
                ],
            },
        },
    },
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_merged_defaults",
    defaults: ["library_linking_strategy_lib_a_defaults"],
    host_supported: true,
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {
                static_libs: [
                    "lib_b",
                ],
            },
            conditions_default: {
                shared_libs: [
                    "lib_b",
                ],
            },
        },
    },
}

cc_binary {
    name: "library_linking_strategy_sample_binary",
    srcs: ["library_linking_strategy.cc"],
    defaults: ["library_linking_strategy_merged_defaults"],
}`

	otherDeps := `
cc_library { name: "lib_a"}
cc_library { name: "lib_b"}
cc_library { name: "lib_default"}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem: map[string]string{
			"foo/bar/Android.bp": otherDeps,
		},
		StubbedBuildDefinitions: []string{"//foo/bar:lib_a", "//foo/bar:lib_b", "//foo/bar:lib_default"},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "library_linking_strategy_sample_binary",
    deps = select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [
            "//foo/bar:lib_b_bp2build_cc_library_static",
            "//foo/bar:lib_a_bp2build_cc_library_static",
        ],
        "//conditions:default": [],
    }),
    dynamic_deps = select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [],
        "//conditions:default": [
            "//foo/bar:lib_b",
            "//foo/bar:lib_a",
        ],
    }),
    local_includes = ["."],
    srcs = ["library_linking_strategy.cc"],
)`}})
}

func TestSoongConfigModuleType_Defaults_Another(t *testing.T) {
	bp := `
soong_config_string_variable {
    name: "library_linking_strategy",
    values: [
        "prefer_static",
    ],
}

soong_config_module_type {
    name: "library_linking_strategy_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "ANDROID",
    variables: ["library_linking_strategy"],
    properties: [
        "shared_libs",
        "static_libs",
    ],
}

library_linking_strategy_cc_defaults {
    name: "library_linking_strategy_sample_defaults",
    soong_config_variables: {
        library_linking_strategy: {
            prefer_static: {
                static_libs: [
                    "lib_a",
                    "lib_b",
                ],
            },
            conditions_default: {
                shared_libs: [
                    "lib_a",
                    "lib_b",
                ],
            },
        },
    },
}

cc_binary {
    name: "library_linking_strategy_sample_binary",
    host_supported: true,
    srcs: ["library_linking_strategy.cc"],
    defaults: ["library_linking_strategy_sample_defaults"],
}`

	otherDeps := `
cc_library { name: "lib_a"}
cc_library { name: "lib_b"}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		StubbedBuildDefinitions:    []string{"//foo/bar:lib_a", "//foo/bar:lib_b"},
		Filesystem: map[string]string{
			"foo/bar/Android.bp": otherDeps,
		},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "library_linking_strategy_sample_binary",
    deps = select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [
            "//foo/bar:lib_a_bp2build_cc_library_static",
            "//foo/bar:lib_b_bp2build_cc_library_static",
        ],
        "//conditions:default": [],
    }),
    dynamic_deps = select({
        "//build/bazel/product_config/config_settings:android__library_linking_strategy__prefer_static": [],
        "//conditions:default": [
            "//foo/bar:lib_a",
            "//foo/bar:lib_b",
        ],
    }),
    local_includes = ["."],
    srcs = ["library_linking_strategy.cc"],
)`}})
}

func TestSoongConfigModuleType_Defaults_UnusedProps(t *testing.T) {
	bp := `
soong_config_string_variable {
    name: "alphabet",
    values: [
        "a",
        "b",
        "c", // unused
    ],
}

soong_config_module_type {
    name: "alphabet_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "ANDROID",
    variables: ["alphabet"],
    properties: [
        "cflags", // unused
        "shared_libs",
        "static_libs",
    ],
}

alphabet_cc_defaults {
    name: "alphabet_sample_cc_defaults",
    soong_config_variables: {
        alphabet: {
            a: {
                shared_libs: [
                    "lib_a",
                ],
            },
            b: {
                shared_libs: [
                    "lib_b",
                ],
            },
            conditions_default: {
                static_libs: [
                    "lib_default",
                ],
            },
        },
    },
}

cc_binary {
    name: "alphabet_binary",
    host_supported: true,
    srcs: ["main.cc"],
    defaults: ["alphabet_sample_cc_defaults"],
}`

	otherDeps := `
cc_library { name: "lib_a"}
cc_library { name: "lib_b"}
cc_library { name: "lib_default"}
`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem: map[string]string{
			"foo/bar/Android.bp": otherDeps,
		},
		StubbedBuildDefinitions: []string{"//foo/bar:lib_a", "//foo/bar:lib_b", "//foo/bar:lib_default"},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "alphabet_binary",
    deps = select({
        "//build/bazel/product_config/config_settings:android__alphabet__a": [],
        "//build/bazel/product_config/config_settings:android__alphabet__b": [],
        "//conditions:default": ["//foo/bar:lib_default_bp2build_cc_library_static"],
    }),
    dynamic_deps = select({
        "//build/bazel/product_config/config_settings:android__alphabet__a": ["//foo/bar:lib_a"],
        "//build/bazel/product_config/config_settings:android__alphabet__b": ["//foo/bar:lib_b"],
        "//conditions:default": [],
    }),
    local_includes = ["."],
    srcs = ["main.cc"],
)`}})
}

func TestSoongConfigModuleType_ProductVariableConfigWithPlatformConfig(t *testing.T) {
	bp := `
soong_config_bool_variable {
    name: "special_build",
}

soong_config_module_type {
    name: "alphabet_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "alphabet_module",
    bool_variables: ["special_build"],
    properties: ["enabled"],
}

alphabet_cc_defaults {
    name: "alphabet_sample_cc_defaults",
    soong_config_variables: {
        special_build: {
            enabled: true,
        },
    },
}

cc_binary {
    name: "alphabet_binary",
    srcs: ["main.cc"],
    host_supported: true,
    defaults: ["alphabet_sample_cc_defaults"],
    enabled: false,
    arch: {
        x86_64: {
            enabled: false,
        },
    },
    target: {
        darwin: {
            enabled: false,
        },
    },
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem:                 map[string]string{},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "alphabet_binary",
    local_includes = ["."],
    srcs = ["main.cc"],
    target_compatible_with = select({
        "//build/bazel_common_rules/platforms/os_arch:android_x86_64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:darwin_arm64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:darwin_x86_64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:linux_bionic_x86_64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:linux_glibc_x86_64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:linux_musl_x86_64": ["@platforms//:incompatible"],
        "//build/bazel_common_rules/platforms/os_arch:windows_x86_64": ["@platforms//:incompatible"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel/product_config/config_settings:alphabet_module__special_build": [],
        "//conditions:default": ["@platforms//:incompatible"],
    }),
)`}})
}

func TestSoongConfigModuleType_ProductVariableConfigOverridesEnable(t *testing.T) {
	bp := `
soong_config_bool_variable {
    name: "special_build",
}

soong_config_module_type {
    name: "alphabet_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "alphabet_module",
    bool_variables: ["special_build"],
    properties: ["enabled"],
}

alphabet_cc_defaults {
    name: "alphabet_sample_cc_defaults",
    soong_config_variables: {
        special_build: {
            enabled: true,
        },
    },
}

cc_binary {
    name: "alphabet_binary",
    srcs: ["main.cc"],
    defaults: ["alphabet_sample_cc_defaults"],
    enabled: false,
}

alphabet_cc_defaults {
    name: "alphabet_sample_cc_defaults_conditions_default",
    soong_config_variables: {
        special_build: {
		conditions_default: {
			enabled: false,
		},
	},
    },
}

cc_binary {
    name: "alphabet_binary_conditions_default",
    srcs: ["main.cc"],
    defaults: ["alphabet_sample_cc_defaults_conditions_default"],
    enabled: false,
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem:                 map[string]string{},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "alphabet_binary",
    local_includes = ["."],
    srcs = ["main.cc"],
    target_compatible_with = select({
        "//build/bazel/product_config/config_settings:alphabet_module__special_build": [],
        "//conditions:default": ["@platforms//:incompatible"],
    }),
)`,
			MakeBazelTarget("cc_binary", "alphabet_binary_conditions_default", AttrNameToString{
				"local_includes":         `["."]`,
				"srcs":                   `["main.cc"]`,
				"target_compatible_with": `["@platforms//:incompatible"]`,
			}),
		}})
}

func TestSoongConfigModuleType_ProductVariableIgnoredIfEnabledByDefault(t *testing.T) {
	bp := `
soong_config_bool_variable {
    name: "special_build",
}

soong_config_module_type {
    name: "alphabet_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "alphabet_module",
    bool_variables: ["special_build"],
    properties: ["enabled"],
}

alphabet_cc_defaults {
    name: "alphabet_sample_cc_defaults",
    host_supported: true,
    soong_config_variables: {
        special_build: {
            enabled: true,
        },
    },
}

cc_binary {
    name: "alphabet_binary",
    srcs: ["main.cc"],
    defaults: ["alphabet_sample_cc_defaults"],
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem:                 map[string]string{},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "alphabet_binary",
    local_includes = ["."],
    srcs = ["main.cc"],
)`}})
}

func TestSoongConfigModuleType_CombinedWithArchVariantProperties(t *testing.T) {
	bp := `
soong_config_bool_variable {
    name: "my_bool_variable",
}

soong_config_string_variable {
    name: "my_string_variable",
    values: [
        "value1",
        "value2",
    ],
}

soong_config_module_type {
    name: "special_build_cc_defaults",
    module_type: "cc_defaults",
    config_namespace: "my_namespace",
    bool_variables: ["my_bool_variable"],
    variables: ["my_string_variable"],
    properties: ["target.android.cflags", "cflags"],
}

special_build_cc_defaults {
    name: "sample_cc_defaults",
    target: {
        android: {
            cflags: ["-DFOO"],
        },
    },
    soong_config_variables: {
        my_bool_variable: {
            target: {
                android: {
                    cflags: ["-DBAR"],
                },
            },
            conditions_default: {
                target: {
                    android: {
                        cflags: ["-DBAZ"],
                    },
                },
            },
        },
        my_string_variable: {
            value1: {
                cflags: ["-DVALUE1_NOT_ANDROID"],
                target: {
                    android: {
                        cflags: ["-DVALUE1"],
                    },
                },
            },
            value2: {
                target: {
                    android: {
                        cflags: ["-DVALUE2"],
                    },
                },
            },
            conditions_default: {
                target: {
                    android: {
                        cflags: ["-DSTRING_VAR_CONDITIONS_DEFAULT"],
                    },
                },
            },
        },
    },
}

cc_binary {
    name: "my_binary",
    srcs: ["main.cc"],
    defaults: ["sample_cc_defaults"],
}`

	runSoongConfigModuleTypeTest(t, Bp2buildTestCase{
		Description:                "soong config variables - generates selects for library_linking_strategy",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		Filesystem:                 map[string]string{},
		ExpectedBazelTargets: []string{`cc_binary(
    name = "my_binary",
    copts = select({
        "//build/bazel_common_rules/platforms/os:android": ["-DFOO"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel/product_config/config_settings:my_namespace__my_bool_variable__android": ["-DBAR"],
        "//build/bazel/product_config/config_settings:my_namespace__my_bool_variable__conditions_default__android": ["-DBAZ"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel/product_config/config_settings:my_namespace__my_string_variable__value1": ["-DVALUE1_NOT_ANDROID"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel/product_config/config_settings:my_namespace__my_string_variable__conditions_default__android": ["-DSTRING_VAR_CONDITIONS_DEFAULT"],
        "//build/bazel/product_config/config_settings:my_namespace__my_string_variable__value1__android": ["-DVALUE1"],
        "//build/bazel/product_config/config_settings:my_namespace__my_string_variable__value2__android": ["-DVALUE2"],
        "//conditions:default": [],
    }),
    local_includes = ["."],
    srcs = ["main.cc"],
    target_compatible_with = ["//build/bazel_common_rules/platforms/os:android"],
)`}})
}

// If we have
// A. a soong_config_module_type with target.android_<arch>.* in properties
// B. a module that uses this module type but does not set target.android_<arch>.* via soong config vars
// Then we should not panic
func TestPanicsIfSoongConfigModuleTypeHasArchSpecificProperties(t *testing.T) {
	commonBp := `
soong_config_bool_variable {
	name: "my_bool_variable",
}
soong_config_module_type {
	name: "special_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "my_namespace",
	bool_variables: ["my_bool_variable"],
	properties: [
		"cflags",
		"target.android_arm64.shared_libs",
	],
}
cc_binary {
	name: "my_binary",
	defaults: ["my_special_cc_defaults"],
}
`
	testCases := []struct {
		desc            string
		additionalBp    string
		isPanicExpected bool
	}{
		{
			desc: "target.android_arm64 is not set, bp2build should not panic",
			additionalBp: `
special_cc_defaults {
	name: "my_special_cc_defaults",
	soong_config_variables: {
		my_bool_variable: {
			cflags: ["-DFOO"],
			conditions_default: {
				cflags: ["-DBAR"],
			}
		}
	},
}
			`,
			isPanicExpected: false,
		},
		{
			desc: "target.android_arm64 is set using the bool soong config var, bp2build should panic",
			additionalBp: `
special_cc_defaults {
	name: "my_special_cc_defaults",
	soong_config_variables: {
		my_bool_variable: {
			cflags: ["-DFOO"],
			target: {
				android_arm64: {
					shared_libs: ["liblog"],
				},
			},
			conditions_default: {
				cflags: ["-DBAR"],
			}
		}
	},
}
			`,
			isPanicExpected: true,
		},
		{
			desc: "target.android_arm64 is set using conditions_default for the bool soong config var, bp2build should panic",
			additionalBp: `
special_cc_defaults {
	name: "my_special_cc_defaults",
	soong_config_variables: {
		my_bool_variable: {
			cflags: ["-DFOO"],
			conditions_default: {
				cflags: ["-DBAR"],
				target: {
					android_arm64: {
						shared_libs: ["liblog"],
					},
				},
			}
		}
	},
}
			`,
			isPanicExpected: true,
		},
	}
	for _, tc := range testCases {
		bp2buildTestCase := Bp2buildTestCase{
			Description:                tc.desc,
			ModuleTypeUnderTest:        "cc_binary",
			ModuleTypeUnderTestFactory: cc.BinaryFactory,
			Blueprint:                  commonBp + tc.additionalBp,
			// Check in `foo` dir so that we can check whether it panics or not and not trip over an empty `ExpectedBazelTargets`
			Dir:                  "foo",
			ExpectedBazelTargets: []string{},
		}
		if tc.isPanicExpected {
			bp2buildTestCase.ExpectedErr = fmt.Errorf("TODO: support other target types in soong config variable structs: Android_arm64")
		}
		runSoongConfigModuleTypeTest(t, bp2buildTestCase)
	}
}

func TestNoPanicIfEnabledIsNotUsed(t *testing.T) {
	bp := `
soong_config_string_variable {
	name: "my_string_variable",
	values: ["val1", "val2"],
}
soong_config_module_type {
	name: "special_cc_defaults",
	module_type: "cc_defaults",
	config_namespace: "my_namespace",
	variables: ["my_string_variable"],
	properties: [
		"cflags",
		"enabled",
	],
}
special_cc_defaults {
	name: "my_special_cc_defaults",
	soong_config_variables: {
		my_string_variable: {
			val1: {
				cflags: ["-DFOO"],
			},
			val2: {
				cflags: ["-DBAR"],
			},
		},
	},
}
cc_binary {
	name: "my_binary",
	enabled: false,
	defaults: ["my_special_cc_defaults"],
}
`
	tc := Bp2buildTestCase{
		Description:                "Soong config vars is not used to set `enabled` property",
		ModuleTypeUnderTest:        "cc_binary",
		ModuleTypeUnderTestFactory: cc.BinaryFactory,
		Blueprint:                  bp,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("cc_binary", "my_binary", AttrNameToString{
				"copts": `select({
        "//build/bazel/product_config/config_settings:my_namespace__my_string_variable__val1": ["-DFOO"],
        "//build/bazel/product_config/config_settings:my_namespace__my_string_variable__val2": ["-DBAR"],
        "//conditions:default": [],
    })`,
				"local_includes":         `["."]`,
				"target_compatible_with": `["@platforms//:incompatible"]`,
			}),
		},
	}
	runSoongConfigModuleTypeTest(t, tc)
}
