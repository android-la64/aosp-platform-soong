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

package android

import (
	"bufio"
	"errors"
	"strings"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"

	"android/soong/android/allowlists"
)

const (
	// A sentinel value to be used as a key in Bp2BuildConfig for modules with
	// no package path. This is also the module dir for top level Android.bp
	// modules.
	Bp2BuildTopLevel = "."
)

// FileGroupAsLibrary describes a filegroup module that is converted to some library
// such as aidl_library or proto_library.
type FileGroupAsLibrary interface {
	ShouldConvertToAidlLibrary(ctx BazelConversionPathContext) bool
	ShouldConvertToProtoLibrary(ctx BazelConversionPathContext) bool
	GetAidlLibraryLabel(ctx BazelConversionPathContext) string
	GetProtoLibraryLabel(ctx BazelConversionPathContext) string
}

type BazelConversionStatus struct {
	// Information about _all_ bp2build targets generated by this module. Multiple targets are
	// supported as Soong handles some things within a single target that we may choose to split into
	// multiple targets, e.g. renderscript, protos, yacc within a cc module.
	Bp2buildInfo []bp2buildInfo `blueprint:"mutated"`

	// UnconvertedBp2buildDep stores the module names of direct dependency that were not converted to
	// Bazel
	UnconvertedDeps []string `blueprint:"mutated"`

	// MissingBp2buildDep stores the module names of direct dependency that were not found
	MissingDeps []string `blueprint:"mutated"`
}

type bazelModuleProperties struct {
	// The label of the Bazel target replacing this Soong module. When run in conversion mode, this
	// will import the handcrafted build target into the autogenerated file. Note: this may result in
	// a conflict due to duplicate targets if bp2build_available is also set.
	Label *string

	// If true, bp2build will generate the converted Bazel target for this module. Note: this may
	// cause a conflict due to the duplicate targets if label is also set.
	//
	// This is a bool pointer to support tristates: true, false, not set.
	//
	// To opt-in a module, set bazel_module: { bp2build_available: true }
	// To opt-out a module, set bazel_module: { bp2build_available: false }
	// To defer the default setting for the directory, do not set the value.
	Bp2build_available *bool

	// CanConvertToBazel is set via InitBazelModule to indicate that a module type can be converted to
	// Bazel with Bp2build.
	CanConvertToBazel bool `blueprint:"mutated"`
}

// Properties contains common module properties for Bazel migration purposes.
type properties struct {
	// In "Bazel mixed build" mode, this represents the Bazel target replacing
	// this Soong module.
	Bazel_module bazelModuleProperties
}

// namespacedVariableProperties is a map from a string representing a Soong
// config variable namespace, like "android" or "vendor_name" to a slice of
// pointer to a struct containing a single field called Soong_config_variables
// whose value mirrors the structure in the Blueprint file.
type namespacedVariableProperties map[string][]interface{}

// BazelModuleBase contains the property structs with metadata for modules which can be converted to
// Bazel.
type BazelModuleBase struct {
	bazelProperties properties

	// namespacedVariableProperties is used for soong_config_module_type support
	// in bp2build. Soong config modules allow users to set module properties
	// based on custom product variables defined in Android.bp files. These
	// variables are namespaced to prevent clobbering, especially when set from
	// Makefiles.
	namespacedVariableProperties namespacedVariableProperties

	// baseModuleType is set when this module was created from a module type
	// defined by a soong_config_module_type. Every soong_config_module_type
	// "wraps" another module type, e.g. a soong_config_module_type can wrap a
	// cc_defaults to a custom_cc_defaults, or cc_binary to a custom_cc_binary.
	// This baseModuleType is set to the wrapped module type.
	baseModuleType string
}

// Bazelable is specifies the interface for modules that can be converted to Bazel.
type Bazelable interface {
	bazelProps() *properties
	HasHandcraftedLabel() bool
	HandcraftedLabel() string
	GetBazelLabel(ctx BazelConversionPathContext, module blueprint.Module) string
	ShouldConvertWithBp2build(ctx BazelConversionContext) bool
	shouldConvertWithBp2build(ctx bazelOtherModuleContext, module blueprint.Module) bool
	ConvertWithBp2build(ctx TopDownMutatorContext)

	// namespacedVariableProps is a map from a soong config variable namespace
	// (e.g. acme, android) to a map of interfaces{}, which are really
	// reflect.Struct pointers, representing the value of the
	// soong_config_variables property of a module. The struct pointer is the
	// one with the single member called Soong_config_variables, which itself is
	// a struct containing fields for each supported feature in that namespace.
	//
	// The reason for using an slice of interface{} is to support defaults
	// propagation of the struct pointers.
	namespacedVariableProps() namespacedVariableProperties
	setNamespacedVariableProps(props namespacedVariableProperties)
	BaseModuleType() string
	SetBaseModuleType(baseModuleType string)
}

// ApiProvider is implemented by modules that contribute to an API surface
type ApiProvider interface {
	ConvertWithApiBp2build(ctx TopDownMutatorContext)
}

// MixedBuildBuildable is an interface that module types should implement in order
// to be "handled by Bazel" in a mixed build.
type MixedBuildBuildable interface {
	// IsMixedBuildSupported returns true if and only if this module should be
	// "handled by Bazel" in a mixed build.
	// This "escape hatch" allows modules with corner-case scenarios to opt out
	// of being built with Bazel.
	IsMixedBuildSupported(ctx BaseModuleContext) bool

	// QueueBazelCall invokes request-queueing functions on the BazelContext
	// so that these requests are handled when Bazel's cquery is invoked.
	QueueBazelCall(ctx BaseModuleContext)

	// ProcessBazelQueryResponse uses Bazel information (obtained from the BazelContext)
	// to set module fields and providers to propagate this module's metadata upstream.
	// This effectively "bridges the gap" between Bazel and Soong in a mixed build.
	// Soong modules depending on this module should be oblivious to the fact that
	// this module was handled by Bazel.
	ProcessBazelQueryResponse(ctx ModuleContext)
}

// BazelModule is a lightweight wrapper interface around Module for Bazel-convertible modules.
type BazelModule interface {
	Module
	Bazelable
}

// InitBazelModule is a wrapper function that decorates a BazelModule with Bazel-conversion
// properties.
func InitBazelModule(module BazelModule) {
	module.AddProperties(module.bazelProps())
	module.bazelProps().Bazel_module.CanConvertToBazel = true
}

// bazelProps returns the Bazel properties for the given BazelModuleBase.
func (b *BazelModuleBase) bazelProps() *properties {
	return &b.bazelProperties
}

func (b *BazelModuleBase) namespacedVariableProps() namespacedVariableProperties {
	return b.namespacedVariableProperties
}

func (b *BazelModuleBase) setNamespacedVariableProps(props namespacedVariableProperties) {
	b.namespacedVariableProperties = props
}

func (b *BazelModuleBase) BaseModuleType() string {
	return b.baseModuleType
}

func (b *BazelModuleBase) SetBaseModuleType(baseModuleType string) {
	b.baseModuleType = baseModuleType
}

// HasHandcraftedLabel returns whether this module has a handcrafted Bazel label.
func (b *BazelModuleBase) HasHandcraftedLabel() bool {
	return b.bazelProperties.Bazel_module.Label != nil
}

// HandcraftedLabel returns the handcrafted label for this module, or empty string if there is none
func (b *BazelModuleBase) HandcraftedLabel() string {
	return proptools.String(b.bazelProperties.Bazel_module.Label)
}

// GetBazelLabel returns the Bazel label for the given BazelModuleBase.
func (b *BazelModuleBase) GetBazelLabel(ctx BazelConversionPathContext, module blueprint.Module) string {
	if b.HasHandcraftedLabel() {
		return b.HandcraftedLabel()
	}
	if b.ShouldConvertWithBp2build(ctx) {
		return bp2buildModuleLabel(ctx, module)
	}
	return "" // no label for unconverted module
}

type Bp2BuildConversionAllowlist struct {
	// Configure modules in these directories to enable bp2build_available: true or false by default.
	defaultConfig allowlists.Bp2BuildConfig

	// Keep any existing BUILD files (and do not generate new BUILD files) for these directories
	// in the synthetic Bazel workspace.
	keepExistingBuildFile map[string]bool

	// Per-module allowlist to always opt modules into both bp2build and Bazel Dev Mode mixed
	// builds. These modules are usually in directories with many other modules that are not ready
	// for conversion.
	//
	// A module can either be in this list or its directory allowlisted entirely
	// in bp2buildDefaultConfig, but not both at the same time.
	moduleAlwaysConvert map[string]bool

	// Per-module-type allowlist to always opt modules in to both bp2build and
	// Bazel Dev Mode mixed builds when they have the same type as one listed.
	moduleTypeAlwaysConvert map[string]bool

	// Per-module denylist to always opt modules out of bp2build conversion.
	moduleDoNotConvert map[string]bool

	// Per-module denylist of cc_library modules to only generate the static
	// variant if their shared variant isn't ready or buildable by Bazel.
	ccLibraryStaticOnly map[string]bool
}

// GenerateCcLibraryStaticOnly returns whether a cc_library module should only
// generate a static version of itself based on the current global configuration.
func (a Bp2BuildConversionAllowlist) GenerateCcLibraryStaticOnly(moduleName string) bool {
	return a.ccLibraryStaticOnly[moduleName]
}

// NewBp2BuildAllowlist creates a new, empty Bp2BuildConversionAllowlist
// which can be populated using builder pattern Set* methods
func NewBp2BuildAllowlist() Bp2BuildConversionAllowlist {
	return Bp2BuildConversionAllowlist{
		allowlists.Bp2BuildConfig{},
		map[string]bool{},
		map[string]bool{},
		map[string]bool{},
		map[string]bool{},
		map[string]bool{},
	}
}

// SetDefaultConfig copies the entries from defaultConfig into the allowlist
func (a Bp2BuildConversionAllowlist) SetDefaultConfig(defaultConfig allowlists.Bp2BuildConfig) Bp2BuildConversionAllowlist {
	if a.defaultConfig == nil {
		a.defaultConfig = allowlists.Bp2BuildConfig{}
	}
	for k, v := range defaultConfig {
		a.defaultConfig[k] = v
	}

	return a
}

// SetKeepExistingBuildFile copies the entries from keepExistingBuildFile into the allowlist
func (a Bp2BuildConversionAllowlist) SetKeepExistingBuildFile(keepExistingBuildFile map[string]bool) Bp2BuildConversionAllowlist {
	if a.keepExistingBuildFile == nil {
		a.keepExistingBuildFile = map[string]bool{}
	}
	for k, v := range keepExistingBuildFile {
		a.keepExistingBuildFile[k] = v
	}

	return a
}

// SetModuleAlwaysConvertList copies the entries from moduleAlwaysConvert into the allowlist
func (a Bp2BuildConversionAllowlist) SetModuleAlwaysConvertList(moduleAlwaysConvert []string) Bp2BuildConversionAllowlist {
	if a.moduleAlwaysConvert == nil {
		a.moduleAlwaysConvert = map[string]bool{}
	}
	for _, m := range moduleAlwaysConvert {
		a.moduleAlwaysConvert[m] = true
	}

	return a
}

// SetModuleTypeAlwaysConvertList copies the entries from moduleTypeAlwaysConvert into the allowlist
func (a Bp2BuildConversionAllowlist) SetModuleTypeAlwaysConvertList(moduleTypeAlwaysConvert []string) Bp2BuildConversionAllowlist {
	if a.moduleTypeAlwaysConvert == nil {
		a.moduleTypeAlwaysConvert = map[string]bool{}
	}
	for _, m := range moduleTypeAlwaysConvert {
		a.moduleTypeAlwaysConvert[m] = true
	}

	return a
}

// SetModuleDoNotConvertList copies the entries from moduleDoNotConvert into the allowlist
func (a Bp2BuildConversionAllowlist) SetModuleDoNotConvertList(moduleDoNotConvert []string) Bp2BuildConversionAllowlist {
	if a.moduleDoNotConvert == nil {
		a.moduleDoNotConvert = map[string]bool{}
	}
	for _, m := range moduleDoNotConvert {
		a.moduleDoNotConvert[m] = true
	}

	return a
}

// SetCcLibraryStaticOnlyList copies the entries from ccLibraryStaticOnly into the allowlist
func (a Bp2BuildConversionAllowlist) SetCcLibraryStaticOnlyList(ccLibraryStaticOnly []string) Bp2BuildConversionAllowlist {
	if a.ccLibraryStaticOnly == nil {
		a.ccLibraryStaticOnly = map[string]bool{}
	}
	for _, m := range ccLibraryStaticOnly {
		a.ccLibraryStaticOnly[m] = true
	}

	return a
}

// ShouldKeepExistingBuildFileForDir returns whether an existing BUILD file should be
// added to the build symlink forest based on the current global configuration.
func (a Bp2BuildConversionAllowlist) ShouldKeepExistingBuildFileForDir(dir string) bool {
	if _, ok := a.keepExistingBuildFile[dir]; ok {
		// Exact dir match
		return true
	}
	// Check if subtree match
	for prefix, recursive := range a.keepExistingBuildFile {
		if recursive {
			if strings.HasPrefix(dir, prefix+"/") {
				return true
			}
		}
	}
	// Default
	return false
}

var bp2BuildAllowListKey = NewOnceKey("Bp2BuildAllowlist")
var bp2buildAllowlist OncePer

func GetBp2BuildAllowList() Bp2BuildConversionAllowlist {
	return bp2buildAllowlist.Once(bp2BuildAllowListKey, func() interface{} {
		return NewBp2BuildAllowlist().SetDefaultConfig(allowlists.Bp2buildDefaultConfig).
			SetKeepExistingBuildFile(allowlists.Bp2buildKeepExistingBuildFile).
			SetModuleAlwaysConvertList(allowlists.Bp2buildModuleAlwaysConvertList).
			SetModuleTypeAlwaysConvertList(allowlists.Bp2buildModuleTypeAlwaysConvertList).
			SetModuleDoNotConvertList(allowlists.Bp2buildModuleDoNotConvertList).
			SetCcLibraryStaticOnlyList(allowlists.Bp2buildCcLibraryStaticOnlyList)
	}).(Bp2BuildConversionAllowlist)
}

// MixedBuildsEnabled returns true if a module is ready to be replaced by a
// converted or handcrafted Bazel target. As a side effect, calling this
// method will also log whether this module is mixed build enabled for
// metrics reporting.
func MixedBuildsEnabled(ctx BaseModuleContext) bool {
	mixedBuildEnabled := mixedBuildPossible(ctx)
	ctx.Config().LogMixedBuild(ctx, mixedBuildEnabled)
	return mixedBuildEnabled
}

// mixedBuildPossible returns true if a module is ready to be replaced by a
// converted or handcrafted Bazel target.
func mixedBuildPossible(ctx BaseModuleContext) bool {
	if ctx.Os() == Windows {
		// Windows toolchains are not currently supported.
		return false
	}
	if !ctx.Module().Enabled() {
		return false
	}
	if !convertedToBazel(ctx, ctx.Module()) {
		return false
	}
	return ctx.Config().BazelContext.BazelAllowlisted(ctx.Module().Name())
}

// ConvertedToBazel returns whether this module has been converted (with bp2build or manually) to Bazel.
func convertedToBazel(ctx BazelConversionContext, module blueprint.Module) bool {
	b, ok := module.(Bazelable)
	if !ok {
		return false
	}
	return b.shouldConvertWithBp2build(ctx, module) || b.HasHandcraftedLabel()
}

// ShouldConvertWithBp2build returns whether the given BazelModuleBase should be converted with bp2build
func (b *BazelModuleBase) ShouldConvertWithBp2build(ctx BazelConversionContext) bool {
	return b.shouldConvertWithBp2build(ctx, ctx.Module())
}

type bazelOtherModuleContext interface {
	ModuleErrorf(format string, args ...interface{})
	Config() Config
	OtherModuleType(m blueprint.Module) string
	OtherModuleName(m blueprint.Module) string
	OtherModuleDir(m blueprint.Module) string
}

func (b *BazelModuleBase) shouldConvertWithBp2build(ctx bazelOtherModuleContext, module blueprint.Module) bool {
	if !b.bazelProps().Bazel_module.CanConvertToBazel {
		return false
	}

	// In api_bp2build mode, all soong modules that can provide API contributions should be converted
	// This is irrespective of its presence/absence in bp2build allowlists
	if ctx.Config().BuildMode == ApiBp2build {
		_, providesApis := module.(ApiProvider)
		return providesApis
	}

	propValue := b.bazelProperties.Bazel_module.Bp2build_available
	packagePath := ctx.OtherModuleDir(module)

	// Modules in unit tests which are enabled in the allowlist by type or name
	// trigger this conditional because unit tests run under the "." package path
	isTestModule := packagePath == Bp2BuildTopLevel && proptools.BoolDefault(propValue, false)
	if isTestModule {
		return true
	}

	moduleName := module.Name()
	allowlist := ctx.Config().Bp2buildPackageConfig
	moduleNameAllowed := allowlist.moduleAlwaysConvert[moduleName]
	moduleTypeAllowed := allowlist.moduleTypeAlwaysConvert[ctx.OtherModuleType(module)]
	allowlistConvert := moduleNameAllowed || moduleTypeAllowed
	if moduleNameAllowed && moduleTypeAllowed {
		ctx.ModuleErrorf("A module cannot be in moduleAlwaysConvert and also be in moduleTypeAlwaysConvert")
		return false
	}

	if allowlist.moduleDoNotConvert[moduleName] {
		if moduleNameAllowed {
			ctx.ModuleErrorf("a module cannot be in moduleDoNotConvert and also be in moduleAlwaysConvert")
		}
		return false
	}

	// This is a tristate value: true, false, or unset.
	if ok, directoryPath := bp2buildDefaultTrueRecursively(packagePath, allowlist.defaultConfig); ok {
		if moduleNameAllowed {
			ctx.ModuleErrorf("A module cannot be in a directory marked Bp2BuildDefaultTrue"+
				" or Bp2BuildDefaultTrueRecursively and also be in moduleAlwaysConvert. Directory: '%s'"+
				" Module: '%s'", directoryPath, moduleName)
			return false
		}

		// Allow modules to explicitly opt-out.
		return proptools.BoolDefault(propValue, true)
	}

	// Allow modules to explicitly opt-in.
	return proptools.BoolDefault(propValue, allowlistConvert)
}

// bp2buildDefaultTrueRecursively checks that the package contains a prefix from the
// set of package prefixes where all modules must be converted. That is, if the
// package is x/y/z, and the list contains either x, x/y, or x/y/z, this function will
// return true.
//
// However, if the package is x/y, and it matches a Bp2BuildDefaultFalse "x/y" entry
// exactly, this module will return false early.
//
// This function will also return false if the package doesn't match anything in
// the config.
//
// This function will also return the allowlist entry which caused a particular
// package to be enabled. Since packages can be enabled via a recursive declaration,
// the path returned will not always be the same as the one provided.
func bp2buildDefaultTrueRecursively(packagePath string, config allowlists.Bp2BuildConfig) (bool, string) {
	// Check if the package path has an exact match in the config.
	if config[packagePath] == allowlists.Bp2BuildDefaultTrue || config[packagePath] == allowlists.Bp2BuildDefaultTrueRecursively {
		return true, packagePath
	} else if config[packagePath] == allowlists.Bp2BuildDefaultFalse {
		return false, packagePath
	}

	// If not, check for the config recursively.
	packagePrefix := ""
	// e.g. for x/y/z, iterate over x, x/y, then x/y/z, taking the final value from the allowlist.
	for _, part := range strings.Split(packagePath, "/") {
		packagePrefix += part
		if config[packagePrefix] == allowlists.Bp2BuildDefaultTrueRecursively {
			// package contains this prefix and this prefix should convert all modules
			return true, packagePrefix
		}
		// Continue to the next part of the package dir.
		packagePrefix += "/"
	}

	return false, packagePath
}

func registerBp2buildConversionMutator(ctx RegisterMutatorsContext) {
	ctx.TopDown("bp2build_conversion", convertWithBp2build).Parallel()
}

func convertWithBp2build(ctx TopDownMutatorContext) {
	bModule, ok := ctx.Module().(Bazelable)
	if !ok || !bModule.shouldConvertWithBp2build(ctx, ctx.Module()) {
		return
	}

	bModule.ConvertWithBp2build(ctx)
}

func registerApiBp2buildConversionMutator(ctx RegisterMutatorsContext) {
	ctx.TopDown("apiBp2build_conversion", convertWithApiBp2build).Parallel()
}

// Generate API contribution targets if the Soong module provides APIs
func convertWithApiBp2build(ctx TopDownMutatorContext) {
	if m, ok := ctx.Module().(ApiProvider); ok {
		m.ConvertWithApiBp2build(ctx)
	}
}

// GetMainClassInManifest scans the manifest file specified in filepath and returns
// the value of attribute Main-Class in the manifest file if it exists, or returns error.
// WARNING: this is for bp2build converters of java_* modules only.
func GetMainClassInManifest(c Config, filepath string) (string, error) {
	file, err := c.fs.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Main-Class:") {
			return strings.TrimSpace(line[len("Main-Class:"):]), nil
		}
	}

	return "", errors.New("Main-Class is not found.")
}
