// Copyright 2016 Google Inc. All rights reserved.
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

package cc

import (
	"fmt"
	"strings"

	"android/soong/android"
	"android/soong/bazel"
	"android/soong/bazel/cquery"
)

//
// Objects (for crt*.o)
//

func init() {
	android.RegisterModuleType("cc_object", ObjectFactory)
	android.RegisterSdkMemberType(ccObjectSdkMemberType)

}

var ccObjectSdkMemberType = &librarySdkMemberType{
	SdkMemberTypeBase: android.SdkMemberTypeBase{
		PropertyName: "native_objects",
		SupportsSdk:  true,
	},
	prebuiltModuleType: "cc_prebuilt_object",
}

type objectLinker struct {
	*baseLinker
	Properties ObjectLinkerProperties

	// Location of the object in the sysroot. Empty if the object is not
	// included in the NDK.
	ndkSysrootPath android.Path
}

type objectBazelHandler struct {
	module *Module
}

var _ BazelHandler = (*objectBazelHandler)(nil)

func (handler *objectBazelHandler) QueueBazelCall(ctx android.BaseModuleContext, label string) {
	bazelCtx := ctx.Config().BazelContext
	bazelCtx.QueueBazelRequest(label, cquery.GetOutputFiles, android.GetConfigKeyApexVariant(ctx, GetApexConfigKey(ctx)))
}

func (handler *objectBazelHandler) ProcessBazelQueryResponse(ctx android.ModuleContext, label string) {
	bazelCtx := ctx.Config().BazelContext
	objPaths, err := bazelCtx.GetOutputFiles(label, android.GetConfigKeyApexVariant(ctx, GetApexConfigKey(ctx)))
	if err != nil {
		ctx.ModuleErrorf(err.Error())
		return
	}

	if len(objPaths) != 1 {
		ctx.ModuleErrorf("expected exactly one object file for '%s', but got %s", label, objPaths)
		return
	}

	handler.module.outputFile = android.OptionalPathForPath(android.PathForBazelOut(ctx, objPaths[0]))
}

type ObjectLinkerProperties struct {
	// list of static library modules that should only provide headers for this module.
	Static_libs []string `android:"arch_variant,variant_prepend"`

	// list of shared library modules should only provide headers for this module.
	Shared_libs []string `android:"arch_variant,variant_prepend"`

	// list of modules that should only provide headers for this module.
	Header_libs []string `android:"arch_variant,variant_prepend"`

	// list of default libraries that will provide headers for this module.  If unset, generally
	// defaults to libc, libm, and libdl.  Set to [] to prevent using headers from the defaults.
	System_shared_libs []string `android:"arch_variant"`

	// names of other cc_object modules to link into this module using partial linking
	Objs []string `android:"arch_variant"`

	// if set, add an extra objcopy --prefix-symbols= step
	Prefix_symbols *string

	// if set, the path to a linker script to pass to ld -r when combining multiple object files.
	Linker_script *string `android:"path,arch_variant"`

	// Indicates that this module is a CRT object. CRT objects will be split
	// into a variant per-API level between min_sdk_version and current.
	Crt *bool

	// Indicates that this module should not be included in the NDK sysroot.
	// Only applies to CRT objects. Defaults to false.
	Exclude_from_ndk_sysroot *bool
}

func newObject(hod android.HostOrDeviceSupported) *Module {
	module := newBaseModule(hod, android.MultilibBoth)
	module.sanitize = &sanitize{}
	module.stl = &stl{}
	return module
}

// cc_object runs the compiler without running the linker. It is rarely
// necessary, but sometimes used to generate .s files from .c files to use as
// input to a cc_genrule module.
func ObjectFactory() android.Module {
	module := newObject(android.HostAndDeviceSupported)
	module.linker = &objectLinker{
		baseLinker: NewBaseLinker(module.sanitize),
	}
	module.compiler = NewBaseCompiler()
	module.bazelHandler = &objectBazelHandler{module: module}

	// Clang's address-significance tables are incompatible with ld -r.
	module.compiler.appendCflags([]string{"-fno-addrsig"})

	module.sdkMemberTypes = []android.SdkMemberType{ccObjectSdkMemberType}

	module.bazelable = true
	return module.Init()
}

// For bp2build conversion.
type bazelObjectAttributes struct {
	Srcs                bazel.LabelListAttribute
	Srcs_as             bazel.LabelListAttribute
	Hdrs                bazel.LabelListAttribute
	Objs                bazel.LabelListAttribute
	Deps                bazel.LabelListAttribute
	System_dynamic_deps bazel.LabelListAttribute
	Copts               bazel.StringListAttribute
	Asflags             bazel.StringListAttribute
	Local_includes      bazel.StringListAttribute
	Absolute_includes   bazel.StringListAttribute
	Stl                 *string
	Linker_script       bazel.LabelAttribute
	Crt                 *bool
	SdkAttributes
}

// objectBp2Build is the bp2build converter from cc_object modules to the
// Bazel equivalent target, plus any necessary include deps for the cc_object.
func objectBp2Build(ctx android.Bp2buildMutatorContext, m *Module) {
	if m.compiler == nil {
		// a cc_object must have access to the compiler decorator for its props.
		ctx.ModuleErrorf("compiler must not be nil for a cc_object module")
	}

	// Set arch-specific configurable attributes
	baseAttributes := bp2BuildParseBaseProps(ctx, m)
	compilerAttrs := baseAttributes.compilerAttributes
	var objs bazel.LabelListAttribute
	var deps bazel.LabelListAttribute
	systemDynamicDeps := bazel.LabelListAttribute{ForceSpecifyEmptyList: true}

	var linkerScript bazel.LabelAttribute

	for axis, configToProps := range m.GetArchVariantProperties(ctx, &ObjectLinkerProperties{}) {
		for config, props := range configToProps {
			if objectLinkerProps, ok := props.(*ObjectLinkerProperties); ok {
				if objectLinkerProps.Linker_script != nil {
					label := android.BazelLabelForModuleSrcSingle(ctx, *objectLinkerProps.Linker_script)
					linkerScript.SetSelectValue(axis, config, label)
				}
				objs.SetSelectValue(axis, config, android.BazelLabelForModuleDeps(ctx, objectLinkerProps.Objs))
				systemSharedLibs := objectLinkerProps.System_shared_libs
				if len(systemSharedLibs) > 0 {
					systemSharedLibs = android.FirstUniqueStrings(systemSharedLibs)
				}
				systemDynamicDeps.SetSelectValue(axis, config, bazelLabelForSharedDeps(ctx, systemSharedLibs))
				deps.SetSelectValue(axis, config, android.BazelLabelForModuleDeps(ctx, objectLinkerProps.Static_libs))
				deps.SetSelectValue(axis, config, android.BazelLabelForModuleDeps(ctx, objectLinkerProps.Shared_libs))
				deps.SetSelectValue(axis, config, android.BazelLabelForModuleDeps(ctx, objectLinkerProps.Header_libs))
				// static_libs, shared_libs, and header_libs have variant_prepend tag
				deps.Prepend = true
			}
		}
	}
	objs.ResolveExcludes()

	// Don't split cc_object srcs across languages. Doing so would add complexity,
	// and this isn't typically done for cc_object.
	srcs := compilerAttrs.srcs
	srcs.Append(compilerAttrs.cSrcs)

	asFlags := compilerAttrs.asFlags
	if compilerAttrs.asSrcs.IsEmpty() {
		// Skip asflags for BUILD file simplicity if there are no assembly sources.
		asFlags = bazel.MakeStringListAttribute(nil)
	}

	attrs := &bazelObjectAttributes{
		Srcs:                srcs,
		Srcs_as:             compilerAttrs.asSrcs,
		Objs:                objs,
		Deps:                deps,
		System_dynamic_deps: systemDynamicDeps,
		Copts:               compilerAttrs.copts,
		Asflags:             asFlags,
		Local_includes:      compilerAttrs.localIncludes,
		Absolute_includes:   compilerAttrs.absoluteIncludes,
		Stl:                 compilerAttrs.stl,
		Linker_script:       linkerScript,
		Crt:                 m.linker.(*objectLinker).Properties.Crt,
		SdkAttributes:       Bp2BuildParseSdkAttributes(m),
	}

	props := bazel.BazelTargetModuleProperties{
		Rule_class:        "cc_object",
		Bzl_load_location: "//build/bazel/rules/cc:cc_object.bzl",
	}

	tags := android.ApexAvailableTagsWithoutTestApexes(ctx, m)

	ctx.CreateBazelTargetModule(props, android.CommonAttributes{
		Name: m.Name(),
		Tags: tags,
	}, attrs)
}

func (object *objectLinker) appendLdflags(flags []string) {
	panic(fmt.Errorf("appendLdflags on objectLinker not supported"))
}

func (object *objectLinker) linkerProps() []interface{} {
	return []interface{}{&object.Properties}
}

func (*objectLinker) linkerInit(ctx BaseModuleContext) {}

func (object *objectLinker) linkerDeps(ctx DepsContext, deps Deps) Deps {
	deps.HeaderLibs = append(deps.HeaderLibs, object.Properties.Header_libs...)
	deps.SharedLibs = append(deps.SharedLibs, object.Properties.Shared_libs...)
	deps.StaticLibs = append(deps.StaticLibs, object.Properties.Static_libs...)
	deps.ObjFiles = append(deps.ObjFiles, object.Properties.Objs...)

	deps.SystemSharedLibs = object.Properties.System_shared_libs
	if deps.SystemSharedLibs == nil {
		// Provide a default set of shared libraries if system_shared_libs is unspecified.
		// Note: If an empty list [] is specified, it implies that the module declines the
		// default shared libraries.
		deps.SystemSharedLibs = append(deps.SystemSharedLibs, ctx.toolchain().DefaultSharedLibraries()...)
	}
	deps.LateSharedLibs = append(deps.LateSharedLibs, deps.SystemSharedLibs...)
	return deps
}

func (object *objectLinker) linkerFlags(ctx ModuleContext, flags Flags) Flags {
	flags.Global.LdFlags = append(flags.Global.LdFlags, ctx.toolchain().ToolchainLdflags())

	if lds := android.OptionalPathForModuleSrc(ctx, object.Properties.Linker_script); lds.Valid() {
		flags.Local.LdFlags = append(flags.Local.LdFlags, "-Wl,-T,"+lds.String())
		flags.LdFlagsDeps = append(flags.LdFlagsDeps, lds.Path())
	}
	return flags
}

func (object *objectLinker) link(ctx ModuleContext,
	flags Flags, deps PathDeps, objs Objects) android.Path {

	objs = objs.Append(deps.Objs)

	var output android.WritablePath
	builderFlags := flagsToBuilderFlags(flags)
	outputName := ctx.ModuleName()
	if !strings.HasSuffix(outputName, objectExtension) {
		outputName += objectExtension
	}

	// isForPlatform is terribly named and actually means isNotApex.
	if Bool(object.Properties.Crt) &&
		!Bool(object.Properties.Exclude_from_ndk_sysroot) && ctx.useSdk() &&
		ctx.isSdkVariant() && ctx.isForPlatform() {

		output = getVersionedLibraryInstallPath(ctx,
			nativeApiLevelOrPanic(ctx, ctx.sdkVersion())).Join(ctx, outputName)
		object.ndkSysrootPath = output
	} else {
		output = android.PathForModuleOut(ctx, outputName)
	}

	outputFile := output

	if len(objs.objFiles) == 1 && String(object.Properties.Linker_script) == "" {
		if String(object.Properties.Prefix_symbols) != "" {
			transformBinaryPrefixSymbols(ctx, String(object.Properties.Prefix_symbols), objs.objFiles[0],
				builderFlags, output)
		} else {
			ctx.Build(pctx, android.BuildParams{
				Rule:   android.Cp,
				Input:  objs.objFiles[0],
				Output: output,
			})
		}
	} else {
		outputAddrSig := android.PathForModuleOut(ctx, "addrsig", outputName)

		if String(object.Properties.Prefix_symbols) != "" {
			input := android.PathForModuleOut(ctx, "unprefixed", outputName)
			transformBinaryPrefixSymbols(ctx, String(object.Properties.Prefix_symbols), input,
				builderFlags, output)
			output = input
		}

		transformObjsToObj(ctx, objs.objFiles, builderFlags, outputAddrSig, flags.LdFlagsDeps)

		// ld -r reorders symbols and invalidates the .llvm_addrsig section, which then causes warnings
		// if the resulting object is used with ld --icf=safe.  Strip the .llvm_addrsig section to
		// prevent the warnings.
		transformObjectNoAddrSig(ctx, outputAddrSig, output)
	}

	ctx.CheckbuildFile(outputFile)
	return outputFile
}

func (object *objectLinker) linkerSpecifiedDeps(specifiedDeps specifiedDeps) specifiedDeps {
	specifiedDeps.sharedLibs = append(specifiedDeps.sharedLibs, object.Properties.Shared_libs...)

	// Must distinguish nil and [] in system_shared_libs - ensure that [] in
	// either input list doesn't come out as nil.
	if specifiedDeps.systemSharedLibs == nil {
		specifiedDeps.systemSharedLibs = object.Properties.System_shared_libs
	} else {
		specifiedDeps.systemSharedLibs = append(specifiedDeps.systemSharedLibs, object.Properties.System_shared_libs...)
	}

	return specifiedDeps
}

func (object *objectLinker) unstrippedOutputFilePath() android.Path {
	return nil
}

func (object *objectLinker) nativeCoverage() bool {
	return true
}

func (object *objectLinker) coverageOutputFilePath() android.OptionalPath {
	return android.OptionalPath{}
}

func (object *objectLinker) object() bool {
	return true
}

func (object *objectLinker) isCrt() bool {
	return Bool(object.Properties.Crt)
}
