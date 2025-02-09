// Copyright 2021 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package android

import (
	"fmt"
	"testing"

	"android/soong/android/allowlists"
	"android/soong/bazel"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"
)

func TestConvertAllModulesInPackage(t *testing.T) {
	testCases := []struct {
		prefixes   allowlists.Bp2BuildConfig
		packageDir string
	}{
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a/b": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a/b":   allowlists.Bp2BuildDefaultTrueRecursively,
				"a/b/c": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":     allowlists.Bp2BuildDefaultTrueRecursively,
				"d/e/f": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":     allowlists.Bp2BuildDefaultFalse,
				"a/b":   allowlists.Bp2BuildDefaultTrueRecursively,
				"a/b/c": allowlists.Bp2BuildDefaultFalse,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":     allowlists.Bp2BuildDefaultTrueRecursively,
				"a/b":   allowlists.Bp2BuildDefaultFalse,
				"a/b/c": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":   allowlists.Bp2BuildDefaultFalseRecursively,
				"a/b": allowlists.Bp2BuildDefaultTrue,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":   allowlists.Bp2BuildDefaultFalseRecursively,
				"a/b": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a/b/c",
		},
	}

	for _, test := range testCases {
		if ok, _ := bp2buildDefaultTrueRecursively(test.packageDir, test.prefixes); !ok {
			t.Errorf("Expected to convert all modules in %s based on %v, but failed.", test.packageDir, test.prefixes)
		}
	}
}

func TestModuleOptIn(t *testing.T) {
	testCases := []struct {
		prefixes   allowlists.Bp2BuildConfig
		packageDir string
	}{
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a/b": allowlists.Bp2BuildDefaultFalse,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":   allowlists.Bp2BuildDefaultFalse,
				"a/b": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a/b": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a", // opt-in by default
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a/b/c": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":     allowlists.Bp2BuildDefaultTrueRecursively,
				"d/e/f": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "foo/bar",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":     allowlists.Bp2BuildDefaultTrueRecursively,
				"a/b":   allowlists.Bp2BuildDefaultFalse,
				"a/b/c": allowlists.Bp2BuildDefaultTrueRecursively,
			},
			packageDir: "a/b",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":     allowlists.Bp2BuildDefaultFalse,
				"a/b":   allowlists.Bp2BuildDefaultTrueRecursively,
				"a/b/c": allowlists.Bp2BuildDefaultFalse,
			},
			packageDir: "a",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":   allowlists.Bp2BuildDefaultFalseRecursively,
				"a/b": allowlists.Bp2BuildDefaultTrue,
			},
			packageDir: "a/b/c",
		},
		{
			prefixes: allowlists.Bp2BuildConfig{
				"a":   allowlists.Bp2BuildDefaultTrueRecursively,
				"a/b": allowlists.Bp2BuildDefaultFalseRecursively,
			},
			packageDir: "a/b/c",
		},
	}

	for _, test := range testCases {
		if ok, _ := bp2buildDefaultTrueRecursively(test.packageDir, test.prefixes); ok {
			t.Errorf("Expected to allow module opt-in in %s based on %v, but failed.", test.packageDir, test.prefixes)
		}
	}
}

type TestBazelModule struct {
	bazel.TestModuleInfo
	BazelModuleBase
}

var _ blueprint.Module = TestBazelModule{}

func (m TestBazelModule) Name() string {
	return m.TestModuleInfo.ModuleName
}

func (m TestBazelModule) GenerateBuildActions(blueprint.ModuleContext) {
}

type TestBazelConversionContext struct {
	omc       bazel.OtherModuleTestContext
	allowlist Bp2BuildConversionAllowlist
	errors    []string
}

var _ bazelOtherModuleContext = &TestBazelConversionContext{}

func (bcc *TestBazelConversionContext) OtherModuleType(m blueprint.Module) string {
	return bcc.omc.OtherModuleType(m)
}

func (bcc *TestBazelConversionContext) OtherModuleName(m blueprint.Module) string {
	return bcc.omc.OtherModuleName(m)
}

func (bcc *TestBazelConversionContext) OtherModuleDir(m blueprint.Module) string {
	return bcc.omc.OtherModuleDir(m)
}

func (bcc *TestBazelConversionContext) ModuleErrorf(format string, args ...interface{}) {
	bcc.errors = append(bcc.errors, fmt.Sprintf(format, args...))
}

func (bcc *TestBazelConversionContext) Config() Config {
	return Config{
		&config{
			Bp2buildPackageConfig: bcc.allowlist,
		},
	}
}

var bazelableBazelModuleBase = BazelModuleBase{
	bazelProperties: properties{
		Bazel_module: BazelModuleProperties{
			CanConvertToBazel: true,
		},
	},
}

func TestBp2BuildAllowlist(t *testing.T) {
	testCases := []struct {
		description    string
		shouldConvert  bool
		expectedErrors []string
		module         TestBazelModule
		allowlist      Bp2BuildConversionAllowlist
	}{
		{
			description:   "allowlist enables module",
			shouldConvert: true,
			module: TestBazelModule{
				TestModuleInfo: bazel.TestModuleInfo{
					ModuleName: "foo",
					Typ:        "rule1",
					Dir:        "dir1",
				},
				BazelModuleBase: bazelableBazelModuleBase,
			},
			allowlist: Bp2BuildConversionAllowlist{
				moduleAlwaysConvert: map[string]bool{
					"foo": true,
				},
			},
		},
		{
			description:    "module in name allowlist and type allowlist fails",
			shouldConvert:  false,
			expectedErrors: []string{"A module \"foo\" of type \"rule1\" cannot be in moduleAlwaysConvert and also be in moduleTypeAlwaysConvert"},
			module: TestBazelModule{
				TestModuleInfo: bazel.TestModuleInfo{
					ModuleName: "foo",
					Typ:        "rule1",
					Dir:        "dir1",
				},
				BazelModuleBase: bazelableBazelModuleBase,
			},
			allowlist: Bp2BuildConversionAllowlist{
				moduleAlwaysConvert: map[string]bool{
					"foo": true,
				},
				moduleTypeAlwaysConvert: map[string]bool{
					"rule1": true,
				},
			},
		},
		{
			description:    "module in allowlist and denylist fails",
			shouldConvert:  false,
			expectedErrors: []string{"a module \"foo\" cannot be in moduleDoNotConvert and also be in moduleAlwaysConvert"},
			module: TestBazelModule{
				TestModuleInfo: bazel.TestModuleInfo{
					ModuleName: "foo",
					Typ:        "rule1",
					Dir:        "dir1",
				},
				BazelModuleBase: bazelableBazelModuleBase,
			},
			allowlist: Bp2BuildConversionAllowlist{
				moduleAlwaysConvert: map[string]bool{
					"foo": true,
				},
				moduleDoNotConvert: map[string]bool{
					"foo": true,
				},
			},
		},
		{
			description:    "module allowlist and enabled directory",
			shouldConvert:  false,
			expectedErrors: []string{"A module cannot be in a directory marked Bp2BuildDefaultTrue or Bp2BuildDefaultTrueRecursively and also be in moduleAlwaysConvert. Directory: 'existing/build/dir' Module: 'foo'"},
			module: TestBazelModule{
				TestModuleInfo: bazel.TestModuleInfo{
					ModuleName: "foo",
					Typ:        "rule1",
					Dir:        "existing/build/dir",
				},
				BazelModuleBase: bazelableBazelModuleBase,
			},
			allowlist: Bp2BuildConversionAllowlist{
				moduleAlwaysConvert: map[string]bool{
					"foo": true,
				},
				defaultConfig: allowlists.Bp2BuildConfig{
					"existing/build/dir": allowlists.Bp2BuildDefaultTrue,
				},
			},
		},
		{
			description:    "module allowlist and enabled subdirectory",
			shouldConvert:  false,
			expectedErrors: []string{"A module cannot be in a directory marked Bp2BuildDefaultTrue or Bp2BuildDefaultTrueRecursively and also be in moduleAlwaysConvert. Directory: 'existing/build/dir' Module: 'foo'"},
			module: TestBazelModule{
				TestModuleInfo: bazel.TestModuleInfo{
					ModuleName: "foo",
					Typ:        "rule1",
					Dir:        "existing/build/dir/subdir",
				},
				BazelModuleBase: bazelableBazelModuleBase,
			},
			allowlist: Bp2BuildConversionAllowlist{
				moduleAlwaysConvert: map[string]bool{
					"foo": true,
				},
				defaultConfig: allowlists.Bp2BuildConfig{
					"existing/build/dir": allowlists.Bp2BuildDefaultTrueRecursively,
				},
			},
		},
		{
			description:   "module enabled in unit test short-circuits other allowlists",
			shouldConvert: true,
			module: TestBazelModule{
				TestModuleInfo: bazel.TestModuleInfo{
					ModuleName: "foo",
					Typ:        "rule1",
					Dir:        ".",
				},
				BazelModuleBase: BazelModuleBase{
					bazelProperties: properties{
						Bazel_module: BazelModuleProperties{
							CanConvertToBazel:  true,
							Bp2build_available: proptools.BoolPtr(true),
						},
					},
				},
			},
			allowlist: Bp2BuildConversionAllowlist{
				moduleAlwaysConvert: map[string]bool{
					"foo": true,
				},
				moduleDoNotConvert: map[string]bool{
					"foo": true,
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			bcc := &TestBazelConversionContext{
				omc: bazel.OtherModuleTestContext{
					Modules: []bazel.TestModuleInfo{
						test.module.TestModuleInfo,
					},
				},
				allowlist: test.allowlist,
			}

			shouldConvert := test.module.shouldConvertWithBp2build(bcc,
				shouldConvertParams{
					module:     test.module.TestModuleInfo,
					moduleDir:  test.module.TestModuleInfo.Dir,
					moduleType: test.module.TestModuleInfo.Typ,
					moduleName: test.module.TestModuleInfo.ModuleName,
				},
			)
			if test.shouldConvert != shouldConvert {
				t.Errorf("Module shouldConvert expected to be: %v, but was: %v", test.shouldConvert, shouldConvert)
			}

			errorsMatch := true
			if len(test.expectedErrors) != len(bcc.errors) {
				errorsMatch = false
			} else {
				for i, err := range test.expectedErrors {
					if err != bcc.errors[i] {
						errorsMatch = false
					}
				}
			}
			if !errorsMatch {
				t.Errorf("Expected errors to be: %v, but were: %v", test.expectedErrors, bcc.errors)
			}
		})
	}
}

func TestBp2buildAllowList(t *testing.T) {
	allowlist := GetBp2BuildAllowList()
	for k, v := range allowlists.Bp2buildDefaultConfig {
		if allowlist.defaultConfig[k] != v {
			t.Errorf("bp2build default config of %s: expected: %v, got: %v", k, v, allowlist.defaultConfig[k])
		}
	}
	for k, v := range allowlists.Bp2buildKeepExistingBuildFile {
		if allowlist.keepExistingBuildFile[k] != v {
			t.Errorf("bp2build keep existing build file of %s: expected: %v, got: %v", k, v, allowlist.keepExistingBuildFile[k])
		}
	}
	for _, k := range allowlists.Bp2buildModuleTypeAlwaysConvertList {
		if !allowlist.moduleTypeAlwaysConvert[k] {
			t.Errorf("bp2build module type always convert of %s: expected: true, got: %v", k, allowlist.moduleTypeAlwaysConvert[k])
		}
	}
	for _, k := range allowlists.Bp2buildModuleDoNotConvertList {
		if !allowlist.moduleDoNotConvert[k] {
			t.Errorf("bp2build module do not convert of %s: expected: true, got: %v", k, allowlist.moduleDoNotConvert[k])
		}
	}
}

func TestShouldKeepExistingBuildFileForDir(t *testing.T) {
	allowlist := NewBp2BuildAllowlist()
	// entry "a/b2/c2" is moot because of its parent "a/b2"
	allowlist.SetKeepExistingBuildFile(map[string]bool{"a": false, "a/b1": false, "a/b2": true, "a/b1/c1": true, "a/b2/c2": false})
	truths := []string{"a", "a/b1", "a/b2", "a/b1/c1", "a/b2/c", "a/b2/c2", "a/b2/c2/d"}
	falsities := []string{"a1", "a/b", "a/b1/c"}
	for _, dir := range truths {
		if !allowlist.ShouldKeepExistingBuildFileForDir(dir) {
			t.Errorf("%s expected TRUE but was FALSE", dir)
		}
	}
	for _, dir := range falsities {
		if allowlist.ShouldKeepExistingBuildFileForDir(dir) {
			t.Errorf("%s expected FALSE but was TRUE", dir)
		}
	}
}

type mixedBuildModule struct {
	ModuleBase
	BazelModuleBase
	props struct {
		Deps                     []string
		Mixed_build_incompatible *bool
		QueuedBazelCall          bool `blueprint:"mutated"`
	}
}

type mixedBuildModuleInfo struct {
	QueuedBazelCall bool
}

var mixedBuildModuleProvider = blueprint.NewProvider(mixedBuildModuleInfo{})

func mixedBuildModuleFactory() Module {
	m := &mixedBuildModule{}
	m.AddProperties(&m.props)
	InitAndroidArchModule(m, HostAndDeviceDefault, MultilibBoth)
	InitBazelModule(m)

	return m
}

func (m *mixedBuildModule) ConvertWithBp2build(ctx Bp2buildMutatorContext) {
}

func (m *mixedBuildModule) DepsMutator(ctx BottomUpMutatorContext) {
	ctx.AddDependency(ctx.Module(), installDepTag{}, m.props.Deps...)
}

func (m *mixedBuildModule) GenerateAndroidBuildActions(ctx ModuleContext) {
}

func (m *mixedBuildModule) IsMixedBuildSupported(ctx BaseModuleContext) bool {
	return !proptools.Bool(m.props.Mixed_build_incompatible)
}

func (m *mixedBuildModule) QueueBazelCall(ctx BaseModuleContext) {
	m.props.QueuedBazelCall = true
}

func (m *mixedBuildModule) ProcessBazelQueryResponse(ctx ModuleContext) {
	ctx.SetProvider(mixedBuildModuleProvider, mixedBuildModuleInfo{
		QueuedBazelCall: m.props.QueuedBazelCall,
	})
}

var prepareForMixedBuildTests = FixtureRegisterWithContext(func(ctx RegistrationContext) {
	ctx.RegisterModuleType("deps", mixedBuildModuleFactory)
	RegisterMixedBuildsMutator(ctx)
})

func TestMixedBuildsEnabledForType(t *testing.T) {
	baseBp := `
	deps {
		name: "foo",
		deps: ["bar"],
		target: { windows: { enabled: true } },
		%s
	}
`
	depBp := `
	deps {
		name: "bar",
		target: {
			windows: {
				enabled: true,
			},
		},
	}
`
	testCases := []struct {
		desc               string
		variant            *string
		missingDeps        bool
		extraBpInfo        string
		mixedBuildsEnabled bool
	}{
		{
			desc:               "mixed builds works",
			mixedBuildsEnabled: true,
			extraBpInfo:        `bazel_module: { bp2build_available: true },`,
		},
		{
			desc:               "missing deps",
			missingDeps:        true,
			mixedBuildsEnabled: false,
			extraBpInfo:        `bazel_module: { bp2build_available: true },`,
		},
		{
			desc:               "windows no mixed builds",
			mixedBuildsEnabled: false,
			variant:            proptools.StringPtr("windows_x86"),
			extraBpInfo:        `bazel_module: { bp2build_available: true },`,
		},
		{
			desc:               "mixed builds disabled by type",
			mixedBuildsEnabled: false,
			extraBpInfo: `mixed_build_incompatible: true,
		bazel_module: { bp2build_available: true },`,
		},
		{
			desc:               "mixed builds not bp2build available",
			mixedBuildsEnabled: false,
			extraBpInfo:        `bazel_module: { bp2build_available: false },`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			handlers := GroupFixturePreparers(
				prepareForMixedBuildTests,
				PrepareForTestWithArchMutator,
				FixtureModifyConfig(func(config Config) {
					config.BazelContext = MockBazelContext{
						OutputBaseDir: "base",
					}
					config.Targets[Windows] = []Target{
						{Windows, Arch{ArchType: X86_64}, NativeBridgeDisabled, "", "", true},
						{Windows, Arch{ArchType: X86}, NativeBridgeDisabled, "", "", true},
					}
				}),
			)
			bp := fmt.Sprintf(baseBp, tc.extraBpInfo)
			if tc.missingDeps {
				handlers = GroupFixturePreparers(
					handlers,
					PrepareForTestWithAllowMissingDependencies,
				)
			} else {
				bp += depBp
			}
			result := handlers.RunTestWithBp(t, bp)

			variant := proptools.StringDefault(tc.variant, "android_arm64_armv8-a")

			m := result.ModuleForTests("foo", variant)
			mixedBuildModuleInfo := result.TestContext.ModuleProvider(m.Module(), mixedBuildModuleProvider).(mixedBuildModuleInfo)
			if w, g := tc.mixedBuildsEnabled, mixedBuildModuleInfo.QueuedBazelCall; w != g {
				t.Errorf("Expected mixed builds enabled %t, got mixed builds enabled %t", w, g)
			}
		})
	}
}
