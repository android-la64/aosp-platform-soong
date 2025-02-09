// Copyright 2015 Google Inc. All rights reserved.
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

// A genrule module takes a list of source files ("srcs" property), an optional
// list of tools ("tools" property), and a command line ("cmd" property), to
// generate output files ("out" property).

package genrule

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"android/soong/bazel/cquery"

	"github.com/google/blueprint"
	"github.com/google/blueprint/bootstrap"
	"github.com/google/blueprint/proptools"

	"android/soong/android"
	"android/soong/bazel"
)

func init() {
	RegisterGenruleBuildComponents(android.InitRegistrationContext)
}

// Test fixture preparer that will register most genrule build components.
//
// Singletons and mutators should only be added here if they are needed for a majority of genrule
// module types, otherwise they should be added under a separate preparer to allow them to be
// selected only when needed to reduce test execution time.
//
// Module types do not have much of an overhead unless they are used so this should include as many
// module types as possible. The exceptions are those module types that require mutators and/or
// singletons in order to function in which case they should be kept together in a separate
// preparer.
var PrepareForTestWithGenRuleBuildComponents = android.GroupFixturePreparers(
	android.FixtureRegisterWithContext(RegisterGenruleBuildComponents),
)

// Prepare a fixture to use all genrule module types, mutators and singletons fully.
//
// This should only be used by tests that want to run with as much of the build enabled as possible.
var PrepareForIntegrationTestWithGenrule = android.GroupFixturePreparers(
	PrepareForTestWithGenRuleBuildComponents,
)

func RegisterGenruleBuildComponents(ctx android.RegistrationContext) {
	ctx.RegisterModuleType("genrule_defaults", defaultsFactory)

	ctx.RegisterModuleType("gensrcs", GenSrcsFactory)
	ctx.RegisterModuleType("genrule", GenRuleFactory)

	ctx.FinalDepsMutators(func(ctx android.RegisterMutatorsContext) {
		ctx.BottomUp("genrule_tool_deps", toolDepsMutator).Parallel()
	})
}

var (
	pctx = android.NewPackageContext("android/soong/genrule")

	// Used by gensrcs when there is more than 1 shard to merge the outputs
	// of each shard into a zip file.
	gensrcsMerge = pctx.AndroidStaticRule("gensrcsMerge", blueprint.RuleParams{
		Command:        "${soongZip} -o ${tmpZip} @${tmpZip}.rsp && ${zipSync} -d ${genDir} ${tmpZip}",
		CommandDeps:    []string{"${soongZip}", "${zipSync}"},
		Rspfile:        "${tmpZip}.rsp",
		RspfileContent: "${zipArgs}",
	}, "tmpZip", "genDir", "zipArgs")
)

func init() {
	pctx.Import("android/soong/android")

	pctx.HostBinToolVariable("soongZip", "soong_zip")
	pctx.HostBinToolVariable("zipSync", "zipsync")
}

type SourceFileGenerator interface {
	GeneratedSourceFiles() android.Paths
	GeneratedHeaderDirs() android.Paths
	GeneratedDeps() android.Paths
}

// Alias for android.HostToolProvider
// Deprecated: use android.HostToolProvider instead.
type HostToolProvider interface {
	android.HostToolProvider
}

type hostToolDependencyTag struct {
	blueprint.BaseDependencyTag
	android.LicenseAnnotationToolchainDependencyTag
	label string
}

func (t hostToolDependencyTag) AllowDisabledModuleDependency(target android.Module) bool {
	// Allow depending on a disabled module if it's replaced by a prebuilt
	// counterpart. We get the prebuilt through android.PrebuiltGetPreferred in
	// GenerateAndroidBuildActions.
	return target.IsReplacedByPrebuilt()
}

var _ android.AllowDisabledModuleDependency = (*hostToolDependencyTag)(nil)

type generatorProperties struct {
	// The command to run on one or more input files. Cmd supports substitution of a few variables.
	//
	// Available variables for substitution:
	//
	//  $(location): the path to the first entry in tools or tool_files.
	//  $(location <label>): the path to the tool, tool_file, input or output with name <label>. Use $(location) if <label> refers to a rule that outputs exactly one file.
	//  $(locations <label>): the paths to the tools, tool_files, inputs or outputs with name <label>. Use $(locations) if <label> refers to a rule that outputs two or more files.
	//  $(in): one or more input files.
	//  $(out): a single output file.
	//  $(depfile): a file to which dependencies will be written, if the depfile property is set to true.
	//  $(genDir): the sandbox directory for this tool; contains $(out).
	//  $$: a literal $
	Cmd *string

	// Enable reading a file containing dependencies in gcc format after the command completes
	Depfile *bool

	// name of the modules (if any) that produces the host executable.   Leave empty for
	// prebuilts or scripts that do not need a module to build them.
	Tools []string

	// Local files that are used by the tool
	Tool_files []string `android:"path"`

	// List of directories to export generated headers from
	Export_include_dirs []string

	// list of input files
	Srcs []string `android:"path,arch_variant"`

	// input files to exclude
	Exclude_srcs []string `android:"path,arch_variant"`

	// Enable restat to update the output only if the output is changed
	Write_if_changed *bool
}

type Module struct {
	android.ModuleBase
	android.DefaultableModuleBase
	android.BazelModuleBase
	android.ApexModuleBase

	// For other packages to make their own genrules with extra
	// properties
	Extra interface{}

	// CmdModifier can be set by wrappers around genrule to modify the command, for example to
	// prefix environment variables to it.
	CmdModifier func(ctx android.ModuleContext, cmd string) string

	android.ImageInterface

	properties generatorProperties

	// For the different tasks that genrule and gensrc generate. genrule will
	// generate 1 task, and gensrc will generate 1 or more tasks based on the
	// number of shards the input files are sharded into.
	taskGenerator taskFunc

	rule        blueprint.Rule
	rawCommands []string

	exportedIncludeDirs android.Paths

	outputFiles android.Paths
	outputDeps  android.Paths

	subName string
	subDir  string

	// Collect the module directory for IDE info in java/jdeps.go.
	modulePaths []string
}

var _ android.MixedBuildBuildable = (*Module)(nil)

type taskFunc func(ctx android.ModuleContext, rawCommand string, srcFiles android.Paths) []generateTask

type generateTask struct {
	in          android.Paths
	out         android.WritablePaths
	depFile     android.WritablePath
	copyTo      android.WritablePaths // For gensrcs to set on gensrcsMerge rule.
	genDir      android.WritablePath
	extraTools  android.Paths // dependencies on tools used by the generator
	extraInputs map[string][]string

	cmd string
	// For gensrsc sharding.
	shard  int
	shards int
}

func (g *Module) GeneratedSourceFiles() android.Paths {
	return g.outputFiles
}

func (g *Module) Srcs() android.Paths {
	return append(android.Paths{}, g.outputFiles...)
}

func (g *Module) GeneratedHeaderDirs() android.Paths {
	return g.exportedIncludeDirs
}

func (g *Module) GeneratedDeps() android.Paths {
	return g.outputDeps
}

func (g *Module) OutputFiles(tag string) (android.Paths, error) {
	if tag == "" {
		return append(android.Paths{}, g.outputFiles...), nil
	}
	// otherwise, tag should match one of outputs
	for _, outputFile := range g.outputFiles {
		if outputFile.Rel() == tag {
			return android.Paths{outputFile}, nil
		}
	}
	return nil, fmt.Errorf("unsupported module reference tag %q", tag)
}

var _ android.SourceFileProducer = (*Module)(nil)
var _ android.OutputFileProducer = (*Module)(nil)

func toolDepsMutator(ctx android.BottomUpMutatorContext) {
	if g, ok := ctx.Module().(*Module); ok {
		for _, tool := range g.properties.Tools {
			tag := hostToolDependencyTag{label: tool}
			if m := android.SrcIsModule(tool); m != "" {
				tool = m
			}
			ctx.AddFarVariationDependencies(ctx.Config().BuildOSTarget.Variations(), tag, tool)
		}
	}
}

func (g *Module) ProcessBazelQueryResponse(ctx android.ModuleContext) {
	g.generateCommonBuildActions(ctx)

	label := g.GetBazelLabel(ctx, g)
	bazelCtx := ctx.Config().BazelContext
	filePaths, err := bazelCtx.GetOutputFiles(label, android.GetConfigKey(ctx))
	if err != nil {
		ctx.ModuleErrorf(err.Error())
		return
	}

	var bazelOutputFiles android.Paths
	exportIncludeDirs := map[string]bool{}
	for _, bazelOutputFile := range filePaths {
		bazelOutputFiles = append(bazelOutputFiles, android.PathForBazelOutRelative(ctx, ctx.ModuleDir(), bazelOutputFile))
		exportIncludeDirs[filepath.Dir(bazelOutputFile)] = true
	}
	g.outputFiles = bazelOutputFiles
	g.outputDeps = bazelOutputFiles
	for includePath, _ := range exportIncludeDirs {
		g.exportedIncludeDirs = append(g.exportedIncludeDirs, android.PathForBazelOut(ctx, includePath))
	}
}

// generateCommonBuildActions contains build action generation logic
// common to both the mixed build case and the legacy case of genrule processing.
// To fully support genrule in mixed builds, the contents of this function should
// approach zero; there should be no genrule action registration done directly
// by Soong logic in the mixed-build case.
func (g *Module) generateCommonBuildActions(ctx android.ModuleContext) {
	g.subName = ctx.ModuleSubDir()

	// Collect the module directory for IDE info in java/jdeps.go.
	g.modulePaths = append(g.modulePaths, ctx.ModuleDir())

	if len(g.properties.Export_include_dirs) > 0 {
		for _, dir := range g.properties.Export_include_dirs {
			g.exportedIncludeDirs = append(g.exportedIncludeDirs,
				android.PathForModuleGen(ctx, g.subDir, ctx.ModuleDir(), dir))
			// Also export without ModuleDir for consistency with Export_include_dirs not being set
			g.exportedIncludeDirs = append(g.exportedIncludeDirs,
				android.PathForModuleGen(ctx, g.subDir, dir))
		}
	} else {
		g.exportedIncludeDirs = append(g.exportedIncludeDirs, android.PathForModuleGen(ctx, g.subDir))
	}

	locationLabels := map[string]location{}
	firstLabel := ""

	addLocationLabel := func(label string, loc location) {
		if firstLabel == "" {
			firstLabel = label
		}
		if _, exists := locationLabels[label]; !exists {
			locationLabels[label] = loc
		} else {
			ctx.ModuleErrorf("multiple locations for label %q: %q and %q (do you have duplicate srcs entries?)",
				label, locationLabels[label], loc)
		}
	}

	var tools android.Paths
	var packagedTools []android.PackagingSpec
	if len(g.properties.Tools) > 0 {
		seenTools := make(map[string]bool)

		ctx.VisitDirectDepsBlueprint(func(module blueprint.Module) {
			switch tag := ctx.OtherModuleDependencyTag(module).(type) {
			case hostToolDependencyTag:
				tool := ctx.OtherModuleName(module)
				if m, ok := module.(android.Module); ok {
					// Necessary to retrieve any prebuilt replacement for the tool, since
					// toolDepsMutator runs too late for the prebuilt mutators to have
					// replaced the dependency.
					module = android.PrebuiltGetPreferred(ctx, m)
				}

				switch t := module.(type) {
				case android.HostToolProvider:
					// A HostToolProvider provides the path to a tool, which will be copied
					// into the sandbox.
					if !t.(android.Module).Enabled() {
						if ctx.Config().AllowMissingDependencies() {
							ctx.AddMissingDependencies([]string{tool})
						} else {
							ctx.ModuleErrorf("depends on disabled module %q", tool)
						}
						return
					}
					path := t.HostToolPath()
					if !path.Valid() {
						ctx.ModuleErrorf("host tool %q missing output file", tool)
						return
					}
					if specs := t.TransitivePackagingSpecs(); specs != nil {
						// If the HostToolProvider has PackgingSpecs, which are definitions of the
						// required relative locations of the tool and its dependencies, use those
						// instead.  They will be copied to those relative locations in the sbox
						// sandbox.
						packagedTools = append(packagedTools, specs...)
						// Assume that the first PackagingSpec of the module is the tool.
						addLocationLabel(tag.label, packagedToolLocation{specs[0]})
					} else {
						tools = append(tools, path.Path())
						addLocationLabel(tag.label, toolLocation{android.Paths{path.Path()}})
					}
				case bootstrap.GoBinaryTool:
					// A GoBinaryTool provides the install path to a tool, which will be copied.
					p := android.PathForGoBinary(ctx, t)
					tools = append(tools, p)
					addLocationLabel(tag.label, toolLocation{android.Paths{p}})
				default:
					ctx.ModuleErrorf("%q is not a host tool provider", tool)
					return
				}

				seenTools[tag.label] = true
			}
		})

		// If AllowMissingDependencies is enabled, the build will not have stopped when
		// AddFarVariationDependencies was called on a missing tool, which will result in nonsensical
		// "cmd: unknown location label ..." errors later.  Add a placeholder file to the local label.
		// The command that uses this placeholder file will never be executed because the rule will be
		// replaced with an android.Error rule reporting the missing dependencies.
		if ctx.Config().AllowMissingDependencies() {
			for _, tool := range g.properties.Tools {
				if !seenTools[tool] {
					addLocationLabel(tool, errorLocation{"***missing tool " + tool + "***"})
				}
			}
		}
	}

	if ctx.Failed() {
		return
	}

	for _, toolFile := range g.properties.Tool_files {
		paths := android.PathsForModuleSrc(ctx, []string{toolFile})
		tools = append(tools, paths...)
		addLocationLabel(toolFile, toolLocation{paths})
	}

	addLabelsForInputs := func(propName string, include, exclude []string) android.Paths {
		includeDirInPaths := ctx.DeviceConfig().BuildBrokenInputDir(g.Name())
		var srcFiles android.Paths
		for _, in := range include {
			paths, missingDeps := android.PathsAndMissingDepsRelativeToModuleSourceDir(android.SourceInput{
				Context: ctx, Paths: []string{in}, ExcludePaths: exclude, IncludeDirs: includeDirInPaths,
			})
			if len(missingDeps) > 0 {
				if !ctx.Config().AllowMissingDependencies() {
					panic(fmt.Errorf("should never get here, the missing dependencies %q should have been reported in DepsMutator",
						missingDeps))
				}

				// If AllowMissingDependencies is enabled, the build will not have stopped when
				// the dependency was added on a missing SourceFileProducer module, which will result in nonsensical
				// "cmd: label ":..." has no files" errors later.  Add a placeholder file to the local label.
				// The command that uses this placeholder file will never be executed because the rule will be
				// replaced with an android.Error rule reporting the missing dependencies.
				ctx.AddMissingDependencies(missingDeps)
				addLocationLabel(in, errorLocation{"***missing " + propName + " " + in + "***"})
			} else {
				srcFiles = append(srcFiles, paths...)
				addLocationLabel(in, inputLocation{paths})
			}
		}
		return srcFiles
	}
	srcFiles := addLabelsForInputs("srcs", g.properties.Srcs, g.properties.Exclude_srcs)

	var copyFrom android.Paths
	var outputFiles android.WritablePaths
	var zipArgs strings.Builder

	cmd := String(g.properties.Cmd)
	if g.CmdModifier != nil {
		cmd = g.CmdModifier(ctx, cmd)
	}

	var extraInputs android.Paths
	// Generate tasks, either from genrule or gensrcs.
	for i, task := range g.taskGenerator(ctx, cmd, srcFiles) {
		if len(task.out) == 0 {
			ctx.ModuleErrorf("must have at least one output file")
			return
		}

		// Only handle extra inputs once as these currently are the same across all tasks
		if i == 0 {
			for name, values := range task.extraInputs {
				extraInputs = append(extraInputs, addLabelsForInputs(name, values, []string{})...)
			}
		}

		// Pick a unique path outside the task.genDir for the sbox manifest textproto,
		// a unique rule name, and the user-visible description.
		manifestName := "genrule.sbox.textproto"
		desc := "generate"
		name := "generator"
		if task.shards > 0 {
			manifestName = "genrule_" + strconv.Itoa(task.shard) + ".sbox.textproto"
			desc += " " + strconv.Itoa(task.shard)
			name += strconv.Itoa(task.shard)
		} else if len(task.out) == 1 {
			desc += " " + task.out[0].Base()
		}

		manifestPath := android.PathForModuleOut(ctx, manifestName)

		// Use a RuleBuilder to create a rule that runs the command inside an sbox sandbox.
		rule := getSandboxedRuleBuilder(ctx, android.NewRuleBuilder(pctx, ctx).Sbox(task.genDir, manifestPath))
		if Bool(g.properties.Write_if_changed) {
			rule.Restat()
		}
		cmd := rule.Command()

		for _, out := range task.out {
			addLocationLabel(out.Rel(), outputLocation{out})
		}

		referencedDepfile := false

		rawCommand, err := android.Expand(task.cmd, func(name string) (string, error) {
			// report the error directly without returning an error to android.Expand to catch multiple errors in a
			// single run
			reportError := func(fmt string, args ...interface{}) (string, error) {
				ctx.PropertyErrorf("cmd", fmt, args...)
				return "SOONG_ERROR", nil
			}

			// Apply shell escape to each cases to prevent source file paths containing $ from being evaluated in shell
			switch name {
			case "location":
				if len(g.properties.Tools) == 0 && len(g.properties.Tool_files) == 0 {
					return reportError("at least one `tools` or `tool_files` is required if $(location) is used")
				}
				loc := locationLabels[firstLabel]
				paths := loc.Paths(cmd)
				if len(paths) == 0 {
					return reportError("default label %q has no files", firstLabel)
				} else if len(paths) > 1 {
					return reportError("default label %q has multiple files, use $(locations %s) to reference it",
						firstLabel, firstLabel)
				}
				return proptools.ShellEscape(paths[0]), nil
			case "in":
				return strings.Join(proptools.ShellEscapeList(cmd.PathsForInputs(srcFiles)), " "), nil
			case "out":
				var sandboxOuts []string
				for _, out := range task.out {
					sandboxOuts = append(sandboxOuts, cmd.PathForOutput(out))
				}
				return strings.Join(proptools.ShellEscapeList(sandboxOuts), " "), nil
			case "depfile":
				referencedDepfile = true
				if !Bool(g.properties.Depfile) {
					return reportError("$(depfile) used without depfile property")
				}
				return "__SBOX_DEPFILE__", nil
			case "genDir":
				return proptools.ShellEscape(cmd.PathForOutput(task.genDir)), nil
			default:
				if strings.HasPrefix(name, "location ") {
					label := strings.TrimSpace(strings.TrimPrefix(name, "location "))
					if loc, ok := locationLabels[label]; ok {
						paths := loc.Paths(cmd)
						if len(paths) == 0 {
							return reportError("label %q has no files", label)
						} else if len(paths) > 1 {
							return reportError("label %q has multiple files, use $(locations %s) to reference it",
								label, label)
						}
						return proptools.ShellEscape(paths[0]), nil
					} else {
						return reportError("unknown location label %q is not in srcs, out, tools or tool_files.", label)
					}
				} else if strings.HasPrefix(name, "locations ") {
					label := strings.TrimSpace(strings.TrimPrefix(name, "locations "))
					if loc, ok := locationLabels[label]; ok {
						paths := loc.Paths(cmd)
						if len(paths) == 0 {
							return reportError("label %q has no files", label)
						}
						return proptools.ShellEscape(strings.Join(paths, " ")), nil
					} else {
						return reportError("unknown locations label %q is not in srcs, out, tools or tool_files.", label)
					}
				} else {
					return reportError("unknown variable '$(%s)'", name)
				}
			}
		})

		if err != nil {
			ctx.PropertyErrorf("cmd", "%s", err.Error())
			return
		}

		if Bool(g.properties.Depfile) && !referencedDepfile {
			ctx.PropertyErrorf("cmd", "specified depfile=true but did not include a reference to '${depfile}' in cmd")
			return
		}
		g.rawCommands = append(g.rawCommands, rawCommand)

		cmd.Text(rawCommand)
		cmd.Implicits(srcFiles) // need to be able to reference other srcs
		cmd.Implicits(extraInputs)
		cmd.ImplicitOutputs(task.out)
		cmd.Implicits(task.in)
		cmd.ImplicitTools(tools)
		cmd.ImplicitTools(task.extraTools)
		cmd.ImplicitPackagedTools(packagedTools)
		if Bool(g.properties.Depfile) {
			cmd.ImplicitDepFile(task.depFile)
		}

		// Create the rule to run the genrule command inside sbox.
		rule.Build(name, desc)

		if len(task.copyTo) > 0 {
			// If copyTo is set, multiple shards need to be copied into a single directory.
			// task.out contains the per-shard paths, and copyTo contains the corresponding
			// final path.  The files need to be copied into the final directory by a
			// single rule so it can remove the directory before it starts to ensure no
			// old files remain.  zipsync already does this, so build up zipArgs that
			// zip all the per-shard directories into a single zip.
			outputFiles = append(outputFiles, task.copyTo...)
			copyFrom = append(copyFrom, task.out.Paths()...)
			zipArgs.WriteString(" -C " + task.genDir.String())
			zipArgs.WriteString(android.JoinWithPrefix(task.out.Strings(), " -f "))
		} else {
			outputFiles = append(outputFiles, task.out...)
		}
	}

	if len(copyFrom) > 0 {
		// Create a rule that zips all the per-shard directories into a single zip and then
		// uses zipsync to unzip it into the final directory.
		ctx.Build(pctx, android.BuildParams{
			Rule:        gensrcsMerge,
			Implicits:   copyFrom,
			Outputs:     outputFiles,
			Description: "merge shards",
			Args: map[string]string{
				"zipArgs": zipArgs.String(),
				"tmpZip":  android.PathForModuleGen(ctx, g.subDir+".zip").String(),
				"genDir":  android.PathForModuleGen(ctx, g.subDir).String(),
			},
		})
	}

	g.outputFiles = outputFiles.Paths()
}

func (g *Module) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	// Allowlist genrule to use depfile until we have a solution to remove it.
	// TODO(b/235582219): Remove allowlist for genrule
	if Bool(g.properties.Depfile) {
		sandboxingAllowlistSets := getSandboxingAllowlistSets(ctx)
		// TODO(b/283852474): Checking the GenruleSandboxing flag is temporary in
		// order to pass the presubmit before internal master is updated.
		if ctx.DeviceConfig().GenruleSandboxing() && !sandboxingAllowlistSets.depfileAllowSet[g.Name()] {
			ctx.PropertyErrorf(
				"depfile",
				"Deprecated to ensure the module type is convertible to Bazel. "+
					"Try specifying the dependencies explicitly so that there is no need to use depfile. "+
					"If not possible, the escape hatch is to add the module to allowlists.go to bypass the error.")
		}
	}

	g.generateCommonBuildActions(ctx)

	// For <= 6 outputs, just embed those directly in the users. Right now, that covers >90% of
	// the genrules on AOSP. That will make things simpler to look at the graph in the common
	// case. For larger sets of outputs, inject a phony target in between to limit ninja file
	// growth.
	if len(g.outputFiles) <= 6 {
		g.outputDeps = g.outputFiles
	} else {
		phonyFile := android.PathForModuleGen(ctx, "genrule-phony")
		ctx.Build(pctx, android.BuildParams{
			Rule:   blueprint.Phony,
			Output: phonyFile,
			Inputs: g.outputFiles,
		})
		g.outputDeps = android.Paths{phonyFile}
	}
}

func (g *Module) QueueBazelCall(ctx android.BaseModuleContext) {
	bazelCtx := ctx.Config().BazelContext
	bazelCtx.QueueBazelRequest(g.GetBazelLabel(ctx, g), cquery.GetOutputFiles, android.GetConfigKey(ctx))
}

func (g *Module) IsMixedBuildSupported(ctx android.BaseModuleContext) bool {
	return true
}

// Collect information for opening IDE project files in java/jdeps.go.
func (g *Module) IDEInfo(dpInfo *android.IdeInfo) {
	dpInfo.Srcs = append(dpInfo.Srcs, g.Srcs().Strings()...)
	for _, src := range g.properties.Srcs {
		if strings.HasPrefix(src, ":") {
			src = strings.Trim(src, ":")
			dpInfo.Deps = append(dpInfo.Deps, src)
		}
	}
	dpInfo.Paths = append(dpInfo.Paths, g.modulePaths...)
}

func (g *Module) AndroidMk() android.AndroidMkData {
	return android.AndroidMkData{
		Class:      "ETC",
		OutputFile: android.OptionalPathForPath(g.outputFiles[0]),
		SubName:    g.subName,
		Extra: []android.AndroidMkExtraFunc{
			func(w io.Writer, outputFile android.Path) {
				fmt.Fprintln(w, "LOCAL_UNINSTALLABLE_MODULE := true")
			},
		},
		Custom: func(w io.Writer, name, prefix, moduleDir string, data android.AndroidMkData) {
			android.WriteAndroidMkData(w, data)
			if data.SubName != "" {
				fmt.Fprintln(w, ".PHONY:", name)
				fmt.Fprintln(w, name, ":", name+g.subName)
			}
		},
	}
}

var _ android.ApexModule = (*Module)(nil)

// Implements android.ApexModule
func (g *Module) ShouldSupportSdkVersion(ctx android.BaseModuleContext,
	sdkVersion android.ApiLevel) error {
	// Because generated outputs are checked by client modules(e.g. cc_library, ...)
	// we can safely ignore the check here.
	return nil
}

func generatorFactory(taskGenerator taskFunc, props ...interface{}) *Module {
	module := &Module{
		taskGenerator: taskGenerator,
	}

	module.AddProperties(props...)
	module.AddProperties(&module.properties)

	module.ImageInterface = noopImageInterface{}

	return module
}

type noopImageInterface struct{}

func (x noopImageInterface) ImageMutatorBegin(android.BaseModuleContext)                 {}
func (x noopImageInterface) CoreVariantNeeded(android.BaseModuleContext) bool            { return false }
func (x noopImageInterface) RamdiskVariantNeeded(android.BaseModuleContext) bool         { return false }
func (x noopImageInterface) VendorRamdiskVariantNeeded(android.BaseModuleContext) bool   { return false }
func (x noopImageInterface) DebugRamdiskVariantNeeded(android.BaseModuleContext) bool    { return false }
func (x noopImageInterface) RecoveryVariantNeeded(android.BaseModuleContext) bool        { return false }
func (x noopImageInterface) ExtraImageVariations(ctx android.BaseModuleContext) []string { return nil }
func (x noopImageInterface) SetImageVariation(ctx android.BaseModuleContext, variation string, module android.Module) {
}

func NewGenSrcs() *Module {
	properties := &genSrcsProperties{}

	// finalSubDir is the name of the subdirectory that output files will be generated into.
	// It is used so that per-shard directories can be placed alongside it an then finally
	// merged into it.
	const finalSubDir = "gensrcs"

	taskGenerator := func(ctx android.ModuleContext, rawCommand string, srcFiles android.Paths) []generateTask {
		shardSize := defaultShardSize
		if s := properties.Shard_size; s != nil {
			shardSize = int(*s)
		}

		// gensrcs rules can easily hit command line limits by repeating the command for
		// every input file.  Shard the input files into groups.
		shards := android.ShardPaths(srcFiles, shardSize)
		var generateTasks []generateTask

		for i, shard := range shards {
			var commands []string
			var outFiles android.WritablePaths
			var commandDepFiles []string
			var copyTo android.WritablePaths

			// When sharding is enabled (i.e. len(shards) > 1), the sbox rules for each
			// shard will be write to their own directories and then be merged together
			// into finalSubDir.  If sharding is not enabled (i.e. len(shards) == 1),
			// the sbox rule will write directly to finalSubDir.
			genSubDir := finalSubDir
			if len(shards) > 1 {
				genSubDir = strconv.Itoa(i)
			}

			genDir := android.PathForModuleGen(ctx, genSubDir)
			// TODO(ccross): this RuleBuilder is a hack to be able to call
			// rule.Command().PathForOutput.  Replace this with passing the rule into the
			// generator.
			rule := getSandboxedRuleBuilder(ctx, android.NewRuleBuilder(pctx, ctx).Sbox(genDir, nil))

			for _, in := range shard {
				outFile := android.GenPathWithExt(ctx, finalSubDir, in, String(properties.Output_extension))

				// If sharding is enabled, then outFile is the path to the output file in
				// the shard directory, and copyTo is the path to the output file in the
				// final directory.
				if len(shards) > 1 {
					shardFile := android.GenPathWithExt(ctx, genSubDir, in, String(properties.Output_extension))
					copyTo = append(copyTo, outFile)
					outFile = shardFile
				}

				outFiles = append(outFiles, outFile)

				// pre-expand the command line to replace $in and $out with references to
				// a single input and output file.
				command, err := android.Expand(rawCommand, func(name string) (string, error) {
					switch name {
					case "in":
						return in.String(), nil
					case "out":
						return rule.Command().PathForOutput(outFile), nil
					case "depfile":
						// Generate a depfile for each output file.  Store the list for
						// later in order to combine them all into a single depfile.
						depFile := rule.Command().PathForOutput(outFile.ReplaceExtension(ctx, "d"))
						commandDepFiles = append(commandDepFiles, depFile)
						return depFile, nil
					default:
						return "$(" + name + ")", nil
					}
				})
				if err != nil {
					ctx.PropertyErrorf("cmd", err.Error())
				}

				// escape the command in case for example it contains '#', an odd number of '"', etc
				command = fmt.Sprintf("bash -c %v", proptools.ShellEscape(command))
				commands = append(commands, command)
			}
			fullCommand := strings.Join(commands, " && ")

			var outputDepfile android.WritablePath
			var extraTools android.Paths
			if len(commandDepFiles) > 0 {
				// Each command wrote to a depfile, but ninja can only handle one
				// depfile per rule.  Use the dep_fixer tool at the end of the
				// command to combine all the depfiles into a single output depfile.
				outputDepfile = android.PathForModuleGen(ctx, genSubDir, "gensrcs.d")
				depFixerTool := ctx.Config().HostToolPath(ctx, "dep_fixer")
				fullCommand += fmt.Sprintf(" && %s -o $(depfile) %s",
					rule.Command().PathForTool(depFixerTool),
					strings.Join(commandDepFiles, " "))
				extraTools = append(extraTools, depFixerTool)
			}

			generateTasks = append(generateTasks, generateTask{
				in:         shard,
				out:        outFiles,
				depFile:    outputDepfile,
				copyTo:     copyTo,
				genDir:     genDir,
				cmd:        fullCommand,
				shard:      i,
				shards:     len(shards),
				extraTools: extraTools,
				extraInputs: map[string][]string{
					"data": properties.Data,
				},
			})
		}

		return generateTasks
	}

	g := generatorFactory(taskGenerator, properties)
	g.subDir = finalSubDir
	return g
}

func GenSrcsFactory() android.Module {
	m := NewGenSrcs()
	android.InitAndroidModule(m)
	android.InitBazelModule(m)
	return m
}

type genSrcsProperties struct {
	// extension that will be substituted for each output file
	Output_extension *string

	// maximum number of files that will be passed on a single command line.
	Shard_size *int64

	// Additional files needed for build that are not tooling related.
	Data []string `android:"path"`
}

type bazelGensrcsAttributes struct {
	Srcs             bazel.LabelListAttribute
	Output_extension *string
	Tools            bazel.LabelListAttribute
	Cmd              bazel.StringAttribute
	Data             bazel.LabelListAttribute
}

const defaultShardSize = 50

func NewGenRule() *Module {
	properties := &genRuleProperties{}

	taskGenerator := func(ctx android.ModuleContext, rawCommand string, srcFiles android.Paths) []generateTask {
		outs := make(android.WritablePaths, len(properties.Out))
		var depFile android.WritablePath
		for i, out := range properties.Out {
			outPath := android.PathForModuleGen(ctx, out)
			if i == 0 {
				depFile = outPath.ReplaceExtension(ctx, "d")
			}
			outs[i] = outPath
		}
		return []generateTask{{
			in:      srcFiles,
			out:     outs,
			depFile: depFile,
			genDir:  android.PathForModuleGen(ctx),
			cmd:     rawCommand,
		}}
	}

	return generatorFactory(taskGenerator, properties)
}

func GenRuleFactory() android.Module {
	m := NewGenRule()
	android.InitAndroidModule(m)
	android.InitDefaultableModule(m)
	android.InitBazelModule(m)
	return m
}

type genRuleProperties struct {
	// names of the output files that will be generated
	Out []string
}

type BazelGenruleAttributes struct {
	Srcs  bazel.LabelListAttribute
	Outs  []string
	Tools bazel.LabelListAttribute
	Cmd   bazel.StringAttribute
}

// ConvertWithBp2build converts a Soong module -> Bazel target.
func (m *Module) ConvertWithBp2build(ctx android.Bp2buildMutatorContext) {
	// Bazel only has the "tools" attribute.
	tools_prop := android.BazelLabelForModuleDeps(ctx, m.properties.Tools)
	tool_files_prop := android.BazelLabelForModuleSrc(ctx, m.properties.Tool_files)
	tools_prop.Append(tool_files_prop)

	tools := bazel.MakeLabelListAttribute(tools_prop)
	srcs := bazel.LabelListAttribute{}
	srcs_labels := bazel.LabelList{}
	// Only cc_genrule is arch specific
	if ctx.ModuleType() == "cc_genrule" {
		for axis, configToProps := range m.GetArchVariantProperties(ctx, &generatorProperties{}) {
			for config, props := range configToProps {
				if props, ok := props.(*generatorProperties); ok {
					labels := android.BazelLabelForModuleSrcExcludes(ctx, props.Srcs, props.Exclude_srcs)
					srcs_labels.Append(labels)
					srcs.SetSelectValue(axis, config, labels)
				}
			}
		}
	} else {
		srcs_labels = android.BazelLabelForModuleSrcExcludes(ctx, m.properties.Srcs, m.properties.Exclude_srcs)
		srcs = bazel.MakeLabelListAttribute(srcs_labels)
	}

	var allReplacements bazel.LabelList
	allReplacements.Append(tools.Value)
	allReplacements.Append(bazel.FirstUniqueBazelLabelList(srcs_labels))

	// The Output_extension prop is not in an immediately accessible field
	// in the Module struct, so use GetProperties and cast it
	// to the known struct prop.
	var outputExtension *string
	var data bazel.LabelListAttribute
	if ctx.ModuleType() == "gensrcs" {
		for _, propIntf := range m.GetProperties() {
			if props, ok := propIntf.(*genSrcsProperties); ok {
				outputExtension = props.Output_extension
				dataFiles := android.BazelLabelForModuleSrc(ctx, props.Data)
				allReplacements.Append(bazel.FirstUniqueBazelLabelList(dataFiles))
				data = bazel.MakeLabelListAttribute(dataFiles)
				break
			}
		}
	}

	replaceVariables := func(cmd string) string {
		// Replace in and out variables with $< and $@
		if ctx.ModuleType() == "gensrcs" {
			cmd = strings.ReplaceAll(cmd, "$(in)", "$(SRC)")
			cmd = strings.ReplaceAll(cmd, "$(out)", "$(OUT)")
		} else {
			cmd = strings.Replace(cmd, "$(in)", "$(SRCS)", -1)
			cmd = strings.Replace(cmd, "$(out)", "$(OUTS)", -1)
		}
		cmd = strings.Replace(cmd, "$(genDir)", "$(RULEDIR)", -1)
		if len(tools.Value.Includes) > 0 {
			cmd = strings.Replace(cmd, "$(location)", fmt.Sprintf("$(location %s)", tools.Value.Includes[0].Label), -1)
			cmd = strings.Replace(cmd, "$(locations)", fmt.Sprintf("$(locations %s)", tools.Value.Includes[0].Label), -1)
		}
		for _, l := range allReplacements.Includes {
			bpLoc := fmt.Sprintf("$(location %s)", l.OriginalModuleName)
			bpLocs := fmt.Sprintf("$(locations %s)", l.OriginalModuleName)
			bazelLoc := fmt.Sprintf("$(location %s)", l.Label)
			bazelLocs := fmt.Sprintf("$(locations %s)", l.Label)
			cmd = strings.Replace(cmd, bpLoc, bazelLoc, -1)
			cmd = strings.Replace(cmd, bpLocs, bazelLocs, -1)
		}
		return cmd
	}

	var cmdProp bazel.StringAttribute
	cmdProp.SetValue(replaceVariables(proptools.String(m.properties.Cmd)))
	allProductVariableProps, errs := android.ProductVariableProperties(ctx, m)
	for _, err := range errs {
		ctx.ModuleErrorf("ProductVariableProperties error: %s", err)
	}
	if productVariableProps, ok := allProductVariableProps["Cmd"]; ok {
		for productVariable, value := range productVariableProps {
			var cmd string
			if strValue, ok := value.(*string); ok && strValue != nil {
				cmd = *strValue
			}
			cmd = replaceVariables(cmd)
			cmdProp.SetSelectValue(productVariable.ConfigurationAxis(), productVariable.SelectKey(), &cmd)
		}
	}

	tags := android.ApexAvailableTagsWithoutTestApexes(ctx, m)

	bazelName := m.Name()
	if ctx.ModuleType() == "gensrcs" {
		props := bazel.BazelTargetModuleProperties{
			Rule_class:        "gensrcs",
			Bzl_load_location: "//build/bazel/rules:gensrcs.bzl",
		}
		attrs := &bazelGensrcsAttributes{
			Srcs:             srcs,
			Output_extension: outputExtension,
			Cmd:              cmdProp,
			Tools:            tools,
			Data:             data,
		}
		ctx.CreateBazelTargetModule(props, android.CommonAttributes{
			Name: m.Name(),
			Tags: tags,
		}, attrs)
	} else {
		outs := m.RawOutputFiles(ctx)
		for _, out := range outs {
			if out == bazelName {
				// This is a workaround to circumvent a Bazel warning where a genrule's
				// out may not have the same name as the target itself. This makes no
				// difference for reverse dependencies, because they may depend on the
				// out file by name.
				bazelName = bazelName + "-gen"
				break
			}
		}
		attrs := &BazelGenruleAttributes{
			Srcs:  srcs,
			Outs:  outs,
			Cmd:   cmdProp,
			Tools: tools,
		}
		props := bazel.BazelTargetModuleProperties{
			Rule_class: "genrule",
		}
		ctx.CreateBazelTargetModule(props, android.CommonAttributes{
			Name: bazelName,
			Tags: tags,
		}, attrs)
	}

	if m.needsCcLibraryHeadersBp2build() {
		includeDirs := make([]string, len(m.properties.Export_include_dirs)*2)
		for i, dir := range m.properties.Export_include_dirs {
			includeDirs[i*2] = dir
			includeDirs[i*2+1] = filepath.Clean(filepath.Join(ctx.ModuleDir(), dir))
		}
		attrs := &ccHeaderLibraryAttrs{
			Hdrs:            []string{":" + bazelName},
			Export_includes: includeDirs,
		}
		props := bazel.BazelTargetModuleProperties{
			Rule_class:        "cc_library_headers",
			Bzl_load_location: "//build/bazel/rules/cc:cc_library_headers.bzl",
		}
		ctx.CreateBazelTargetModule(props, android.CommonAttributes{
			Name: m.Name() + genruleHeaderLibrarySuffix,
			Tags: tags,
		}, attrs)
	}
}

const genruleHeaderLibrarySuffix = "__header_library"

func (m *Module) needsCcLibraryHeadersBp2build() bool {
	return len(m.properties.Export_include_dirs) > 0
}

// GenruleCcHeaderMapper is a bazel.LabelMapper function to map genrules to a cc_library_headers
// target when they export multiple include directories.
func GenruleCcHeaderLabelMapper(ctx bazel.OtherModuleContext, label bazel.Label) (string, bool) {
	mod, exists := ctx.ModuleFromName(label.OriginalModuleName)
	if !exists {
		return label.Label, false
	}
	if m, ok := mod.(*Module); ok {
		if m.needsCcLibraryHeadersBp2build() {
			return label.Label + genruleHeaderLibrarySuffix, true
		}
	}
	return label.Label, false
}

type ccHeaderLibraryAttrs struct {
	Hdrs []string

	Export_includes []string
}

// RawOutputFfiles returns the raw outputs specified in Android.bp
// This does not contain the fully resolved path relative to the top of the tree
func (g *Module) RawOutputFiles(ctx android.BazelConversionContext) []string {
	if ctx.Config().BuildMode != android.Bp2build {
		ctx.ModuleErrorf("RawOutputFiles is only supported in bp2build mode")
	}
	// The Out prop is not in an immediately accessible field
	// in the Module struct, so use GetProperties and cast it
	// to the known struct prop.
	var outs []string
	for _, propIntf := range g.GetProperties() {
		if props, ok := propIntf.(*genRuleProperties); ok {
			outs = props.Out
			break
		}
	}
	return outs
}

var Bool = proptools.Bool
var String = proptools.String

// Defaults
type Defaults struct {
	android.ModuleBase
	android.DefaultsModuleBase
}

func defaultsFactory() android.Module {
	return DefaultsFactory()
}

func DefaultsFactory(props ...interface{}) android.Module {
	module := &Defaults{}

	module.AddProperties(props...)
	module.AddProperties(
		&generatorProperties{},
		&genRuleProperties{},
	)

	android.InitDefaultsModule(module)

	return module
}

var sandboxingAllowlistKey = android.NewOnceKey("genruleSandboxingAllowlistKey")

type sandboxingAllowlistSets struct {
	sandboxingDenyModuleSet map[string]bool
	sandboxingDenyPathSet   map[string]bool
	depfileAllowSet         map[string]bool
}

func getSandboxingAllowlistSets(ctx android.PathContext) *sandboxingAllowlistSets {
	return ctx.Config().Once(sandboxingAllowlistKey, func() interface{} {
		sandboxingDenyModuleSet := map[string]bool{}
		sandboxingDenyPathSet := map[string]bool{}
		depfileAllowSet := map[string]bool{}

		android.AddToStringSet(sandboxingDenyModuleSet, append(DepfileAllowList, SandboxingDenyModuleList...))
		android.AddToStringSet(sandboxingDenyPathSet, SandboxingDenyPathList)
		android.AddToStringSet(depfileAllowSet, DepfileAllowList)
		return &sandboxingAllowlistSets{
			sandboxingDenyModuleSet: sandboxingDenyModuleSet,
			sandboxingDenyPathSet:   sandboxingDenyPathSet,
			depfileAllowSet:         depfileAllowSet,
		}
	}).(*sandboxingAllowlistSets)
}

func getSandboxedRuleBuilder(ctx android.ModuleContext, r *android.RuleBuilder) *android.RuleBuilder {
	if !ctx.DeviceConfig().GenruleSandboxing() {
		return r.SandboxTools()
	}
	sandboxingAllowlistSets := getSandboxingAllowlistSets(ctx)
	if sandboxingAllowlistSets.sandboxingDenyPathSet[ctx.ModuleDir()] ||
		sandboxingAllowlistSets.sandboxingDenyModuleSet[ctx.ModuleName()] {
		return r.SandboxTools()
	}
	return r.SandboxInputs()
}
