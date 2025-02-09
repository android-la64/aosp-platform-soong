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
	"android/soong/etc"
)

func runPrebuiltEtcTestCase(t *testing.T, tc Bp2buildTestCase) {
	t.Helper()
	(&tc).ModuleTypeUnderTest = "prebuilt_etc"
	(&tc).ModuleTypeUnderTestFactory = etc.PrebuiltEtcFactory
	RunBp2BuildTestCase(t, registerPrebuiltModuleTypes, tc)
}

func runPrebuiltRootHostTestCase(t *testing.T, tc Bp2buildTestCase) {
	t.Helper()
	(&tc).ModuleTypeUnderTest = "prebuilt_root_host"
	(&tc).ModuleTypeUnderTestFactory = etc.PrebuiltRootHostFactory
	RunBp2BuildTestCase(t, registerPrebuiltModuleTypes, tc)
}

func registerPrebuiltModuleTypes(ctx android.RegistrationContext) {
}

func TestPrebuiltEtcSimple(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - simple example",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "apex_tz_version",
    src: "version/tz_version",
    filename: "tz_version",
    sub_dir: "tz",
    installable: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "apex_tz_version", AttrNameToString{
				"filename":    `"tz_version"`,
				"installable": `False`,
				"src":         `"version/tz_version"`,
				"dir":         `"etc/tz"`,
			})}})
}

func TestPrebuiltEtcArchVariant(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - arch variant",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "apex_tz_version",
    src: "version/tz_version",
    filename: "tz_version",
    sub_dir: "tz",
    installable: false,
    arch: {
      arm: {
        src: "arm",
      },
      arm64: {
        src: "arm64",
      },
    }
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "apex_tz_version", AttrNameToString{
				"filename":    `"tz_version"`,
				"installable": `False`,
				"src": `select({
        "//build/bazel_common_rules/platforms/arch:arm": "arm",
        "//build/bazel_common_rules/platforms/arch:arm64": "arm64",
        "//conditions:default": "version/tz_version",
    })`,
				"dir": `"etc/tz"`,
			})}})
}

func TestPrebuiltEtcArchAndTargetVariant(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - arch variant",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "apex_tz_version",
    src: "version/tz_version",
    filename: "tz_version",
    sub_dir: "tz",
    installable: false,
    arch: {
      arm: {
        src: "arm",
      },
      arm64: {
        src: "darwin_or_arm64",
      },
    },
    target: {
      darwin: {
        src: "darwin_or_arm64",
      }
    },
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "apex_tz_version", AttrNameToString{
				"filename":    `"tz_version"`,
				"installable": `False`,
				"src": `select({
        "//build/bazel_common_rules/platforms/os_arch:android_arm": "arm",
        "//build/bazel_common_rules/platforms/os_arch:android_arm64": "darwin_or_arm64",
        "//build/bazel_common_rules/platforms/os_arch:darwin_arm64": "darwin_or_arm64",
        "//build/bazel_common_rules/platforms/os_arch:darwin_x86_64": "darwin_or_arm64",
        "//build/bazel_common_rules/platforms/os_arch:linux_bionic_arm64": "darwin_or_arm64",
        "//conditions:default": "version/tz_version",
    })`,
				"dir": `"etc/tz"`,
			})}})
}
func TestPrebuiltEtcProductVariables(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt etc - product variables",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "apex_tz_version",
    src: "version/tz_version",
    filename: "tz_version",
    product_variables: {
      native_coverage: {
        src: "src1",
      },
    },
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "apex_tz_version", AttrNameToString{
				"filename": `"tz_version"`,
				"src": `select({
        "//build/bazel/product_config/config_settings:native_coverage": "src1",
        "//conditions:default": "version/tz_version",
    })`,
				"dir": `"etc"`,
			})}})
}

func runPrebuiltUsrShareTestCase(t *testing.T, tc Bp2buildTestCase) {
	t.Helper()
	(&tc).ModuleTypeUnderTest = "prebuilt_usr_share"
	(&tc).ModuleTypeUnderTestFactory = etc.PrebuiltUserShareFactory
	RunBp2BuildTestCase(t, registerPrebuiltModuleTypes, tc)
}

func registerPrebuiltUsrShareModuleTypes(ctx android.RegistrationContext) {
}

func TestPrebuiltUsrShareSimple(t *testing.T) {
	runPrebuiltUsrShareTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_usr_share - simple example",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_usr_share {
    name: "apex_tz_version",
    src: "version/tz_version",
    filename: "tz_version",
    sub_dir: "tz",
    installable: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "apex_tz_version", AttrNameToString{
				"filename":    `"tz_version"`,
				"installable": `False`,
				"src":         `"version/tz_version"`,
				"dir":         `"usr/share/tz"`,
			})}})
}

func TestPrebuiltEtcNoSubdir(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - no subdir",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "apex_tz_version",
    src: "version/tz_version",
    filename: "tz_version",
    installable: false,
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "apex_tz_version", AttrNameToString{
				"filename":    `"tz_version"`,
				"installable": `False`,
				"src":         `"version/tz_version"`,
				"dir":         `"etc"`,
			})}})
}

func TestFilenameAsProperty(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - filename is specified as a property ",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
    src: "fooSrc",
    filename: "fooFileName",
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "foo", AttrNameToString{
				"filename": `"fooFileName"`,
				"src":      `"fooSrc"`,
				"dir":      `"etc"`,
			})}})
}

func TestFileNameFromSrc(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - filename_from_src is true  ",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
    filename_from_src: true,
    src: "fooSrc",
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "foo", AttrNameToString{
				"filename": `"fooSrc"`,
				"src":      `"fooSrc"`,
				"dir":      `"etc"`,
			})}})
}

func TestFileNameFromSrcMultipleSrcs(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - filename_from_src is true but there are multiple srcs",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
    filename_from_src: true,
		arch: {
        arm: {
            src: "barSrc",
        },
        arm64: {
            src: "bazSrc",
        },
	  }
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "foo", AttrNameToString{
				"filename_from_src": `True`,
				"dir":               `"etc"`,
				"src": `select({
        "//build/bazel_common_rules/platforms/arch:arm": "barSrc",
        "//build/bazel_common_rules/platforms/arch:arm64": "bazSrc",
        "//conditions:default": None,
    })`,
			})}})
}

func TestFilenameFromModuleName(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_etc - neither filename nor filename_from_src are specified ",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "foo", AttrNameToString{
				"filename": `"foo"`,
				"dir":      `"etc"`,
			})}})
}

func TestPrebuiltEtcProductVariableArchSrcs(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "prebuilt etc- SRcs from arch variant product variables",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
    filename: "fooFilename",
    arch: {
      arm: {
        src: "armSrc",
        product_variables: {
          native_coverage: {
            src: "nativeCoverageArmSrc",
          },
        },
      },
    },
}`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "foo", AttrNameToString{
				"filename": `"fooFilename"`,
				"dir":      `"etc"`,
				"src": `select({
        "//build/bazel/product_config/config_settings:native_coverage-arm": "nativeCoverageArmSrc",
        "//build/bazel_common_rules/platforms/arch:arm": "armSrc",
        "//conditions:default": None,
    })`,
			})}})
}

func TestPrebuiltEtcProductVariableError(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
    filename: "fooFilename",
    arch: {
      arm: {
        src: "armSrc",
      },
    },
    product_variables: {
      native_coverage: {
        src: "nativeCoverageArmSrc",
      },
    },
}`,
		ExpectedErr: fmt.Errorf("label attribute could not be collapsed"),
	})
}

func TestPrebuiltEtcNoConversionIfSrcEqualsName(t *testing.T) {
	runPrebuiltEtcTestCase(t, Bp2buildTestCase{
		Description: "",
		Filesystem:  map[string]string{},
		Blueprint: `
prebuilt_etc {
    name: "foo",
    filename: "fooFilename",
		src: "foo",
}`,
		ExpectedBazelTargets: []string{},
	})
}

func TestPrebuiltRootHostWithWildCardInSrc(t *testing.T) {
	runPrebuiltRootHostTestCase(t, Bp2buildTestCase{
		Description: "prebuilt_root_host - src string has wild card",
		Filesystem: map[string]string{
			"prh.dat": "",
		},
		Blueprint: `
prebuilt_root_host {
    name: "prh_test",
    src: "*.dat",
    filename_from_src: true,
    relative_install_path: "test/install/path",
    bazel_module: { bp2build_available: true },
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("prebuilt_file", "prh_test", AttrNameToString{
				"filename": `"prh.dat"`,
				"src":      `"prh.dat"`,
				"dir":      `"./test/install/path"`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			})}})
}
