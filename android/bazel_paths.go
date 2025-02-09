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

package android

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"android/soong/bazel"

	"github.com/google/blueprint"
	"github.com/google/blueprint/pathtools"
)

// bazel_paths contains methods to:
//   * resolve Soong path and module references into bazel.LabelList
//   * resolve Bazel path references into Soong-compatible paths
//
// There is often a similar method for Bazel as there is for Soong path handling and should be used
// in similar circumstances
//
//   Bazel                                Soong
//   ==============================================================
//   BazelLabelForModuleSrc               PathForModuleSrc
//   BazelLabelForModuleSrcExcludes       PathForModuleSrcExcludes
//   BazelLabelForModuleDeps              n/a
//   tbd                                  PathForSource
//   tbd                                  ExistentPathsForSources
//   PathForBazelOut                      PathForModuleOut
//
// Use cases:
//  * Module contains a property (often tagged `android:"path"`) that expects paths *relative to the
//    module directory*:
//     * BazelLabelForModuleSrcExcludes, if the module also contains an excludes_<propname> property
//     * BazelLabelForModuleSrc, otherwise
//  * Converting references to other modules to Bazel Labels:
//     BazelLabelForModuleDeps
//  * Converting a path obtained from bazel_handler cquery results:
//     PathForBazelOut
//
// NOTE: all Soong globs are expanded within Soong rather than being converted to a Bazel glob
//       syntax. This occurs because Soong does not have a concept of crossing package boundaries,
//       so the glob as computed by Soong may contain paths that cross package-boundaries. These
//       would be unknowingly omitted if the glob were handled by Bazel. By expanding globs within
//       Soong, we support identification and detection (within Bazel) use of paths that cross
//       package boundaries.
//
// Path resolution:
// * filepath/globs: resolves as itself or is converted to an absolute Bazel label (e.g.
//   //path/to/dir:<filepath>) if path exists in a separate package or subpackage.
// * references to other modules (using the ":name{.tag}" syntax). These resolve as a Bazel label
//   for a target. If the Bazel target is in the local module directory, it will be returned
//   relative to the current package (e.g.  ":<target>"). Otherwise, it will be returned as an
//   absolute Bazel label (e.g.  "//path/to/dir:<target>"). If the reference to another module
//   cannot be resolved,the function will panic. This is often due to the dependency not being added
//   via an AddDependency* method.

// BazelConversionContext is a minimal context interface to check if a module should be converted by bp2build,
// with functions containing information to match against allowlists and denylists.
// If a module is deemed to be convertible by bp2build, then it should rely on a
// BazelConversionPathContext for more functions for dep/path features.
type BazelConversionContext interface {
	Config() Config

	Module() Module
	OtherModuleType(m blueprint.Module) string
	OtherModuleName(m blueprint.Module) string
	OtherModuleDir(m blueprint.Module) string
	ModuleErrorf(format string, args ...interface{})
}

// A subset of the ModuleContext methods which are sufficient to resolve references to paths/deps in
// order to form a Bazel-compatible label for conversion.
type BazelConversionPathContext interface {
	EarlyModulePathContext
	BazelConversionContext

	ModuleName() string
	ModuleType() string
	ModuleErrorf(fmt string, args ...interface{})
	PropertyErrorf(property, fmt string, args ...interface{})
	GetDirectDep(name string) (blueprint.Module, blueprint.DependencyTag)
	ModuleFromName(name string) (blueprint.Module, bool)
	AddUnconvertedBp2buildDep(string)
	AddMissingBp2buildDep(dep string)
}

// BazelLabelForModuleDeps expects a list of reference to other modules, ("<module>"
// or ":<module>") and returns a Bazel-compatible label which corresponds to dependencies on the
// module within the given ctx.
func BazelLabelForModuleDeps(ctx Bp2buildMutatorContext, modules []string) bazel.LabelList {
	return BazelLabelForModuleDepsWithFn(ctx, modules, BazelModuleLabel, true)
}

// BazelLabelForModuleWholeDepsExcludes expects two lists: modules (containing modules to include in
// the list), and excludes (modules to exclude from the list). Both of these should contain
// references to other modules, ("<module>" or ":<module>"). It returns a Bazel-compatible label
// list which corresponds to dependencies on the module within the given ctx, and the excluded
// dependencies.  Prebuilt dependencies will be appended with _alwayslink so they can be handled as
// whole static libraries.
func BazelLabelForModuleDepsExcludes(ctx Bp2buildMutatorContext, modules, excludes []string) bazel.LabelList {
	return BazelLabelForModuleDepsExcludesWithFn(ctx, modules, excludes, BazelModuleLabel)
}

// BazelLabelForModuleDepsWithFn expects a list of reference to other modules, ("<module>"
// or ":<module>") and applies moduleToLabelFn to determine and return a Bazel-compatible label
// which corresponds to dependencies on the module within the given ctx.
func BazelLabelForModuleDepsWithFn(ctx Bp2buildMutatorContext, modules []string,
	moduleToLabelFn func(BazelConversionPathContext, blueprint.Module) string,
	markAsDeps bool) bazel.LabelList {
	var labels bazel.LabelList
	// In some cases, a nil string list is different than an explicitly empty list.
	if len(modules) == 0 && modules != nil {
		labels.Includes = []bazel.Label{}
		return labels
	}
	modules = FirstUniqueStrings(modules)
	for _, module := range modules {
		bpText := module
		if m := SrcIsModule(module); m == "" {
			module = ":" + module
		}
		if m, t := SrcIsModuleWithTag(module); m != "" {
			l := getOtherModuleLabel(ctx, m, t, moduleToLabelFn, markAsDeps)
			if l != nil {
				l.OriginalModuleName = bpText
				labels.Includes = append(labels.Includes, *l)
			}
		} else {
			ctx.ModuleErrorf("%q, is not a module reference", module)
		}
	}
	return labels
}

// BazelLabelForModuleDepsExcludesWithFn expects two lists: modules (containing modules to include in the
// list), and excludes (modules to exclude from the list). Both of these should contain references
// to other modules, ("<module>" or ":<module>"). It applies moduleToLabelFn to determine and return a
// Bazel-compatible label list which corresponds to dependencies on the module within the given ctx, and
// the excluded dependencies.
func BazelLabelForModuleDepsExcludesWithFn(ctx Bp2buildMutatorContext, modules, excludes []string,
	moduleToLabelFn func(BazelConversionPathContext, blueprint.Module) string) bazel.LabelList {
	moduleLabels := BazelLabelForModuleDepsWithFn(ctx, RemoveListFromList(modules, excludes), moduleToLabelFn, true)
	if len(excludes) == 0 {
		return moduleLabels
	}
	excludeLabels := BazelLabelForModuleDepsWithFn(ctx, excludes, moduleToLabelFn, false)
	return bazel.LabelList{
		Includes: moduleLabels.Includes,
		Excludes: excludeLabels.Includes,
	}
}

func BazelLabelForModuleSrcSingle(ctx Bp2buildMutatorContext, path string) bazel.Label {
	if srcs := BazelLabelForModuleSrcExcludes(ctx, []string{path}, []string(nil)).Includes; len(srcs) > 0 {
		return srcs[0]
	}
	return bazel.Label{}
}

func BazelLabelForModuleDepSingle(ctx Bp2buildMutatorContext, path string) bazel.Label {
	if srcs := BazelLabelForModuleDepsExcludes(ctx, []string{path}, []string(nil)).Includes; len(srcs) > 0 {
		return srcs[0]
	}
	return bazel.Label{}
}

// BazelLabelForModuleSrc expects a list of path (relative to local module directory) and module
// references (":<module>") and returns a bazel.LabelList{} containing the resolved references in
// paths, relative to the local module, or Bazel-labels (absolute if in a different package or
// relative if within the same package).
// Properties must have been annotated with struct tag `android:"path"` so that dependencies modules
// will have already been handled by the pathdeps mutator.
func BazelLabelForModuleSrc(ctx Bp2buildMutatorContext, paths []string) bazel.LabelList {
	return BazelLabelForModuleSrcExcludes(ctx, paths, []string(nil))
}

// BazelLabelForModuleSrc expects lists of path and excludes (relative to local module directory)
// and module references (":<module>") and returns a bazel.LabelList{} containing the resolved
// references in paths, minus those in excludes, relative to the local module, or Bazel-labels
// (absolute if in a different package or relative if within the same package).
// Properties must have been annotated with struct tag `android:"path"` so that dependencies modules
// will have already been handled by the pathdeps mutator.
func BazelLabelForModuleSrcExcludes(ctx Bp2buildMutatorContext, paths, excludes []string) bazel.LabelList {
	excludeLabels := expandSrcsForBazel(ctx, excludes, []string(nil), false)
	excluded := make([]string, 0, len(excludeLabels.Includes))
	for _, e := range excludeLabels.Includes {
		excluded = append(excluded, e.Label)
	}
	labels := expandSrcsForBazel(ctx, paths, excluded, true)
	labels.Excludes = excludeLabels.Includes
	labels = TransformSubpackagePaths(ctx.Config(), ctx.ModuleDir(), labels)
	return labels
}

func BazelLabelForSrcPatternExcludes(ctx BazelConversionPathContext, dir, pattern string, excludes []string) bazel.LabelList {
	topRelPaths, err := ctx.GlobWithDeps(filepath.Join(dir, pattern), excludes)
	if err != nil {
		ctx.ModuleErrorf("Could not search dir: %s for pattern %s due to %v\n", dir, pattern, err)
	}
	// An intermediate list of labels relative to `dir` that assumes that there no subpacakges beneath `dir`
	dirRelLabels := []bazel.Label{}
	for _, topRelPath := range topRelPaths {
		dirRelPath := Rel(ctx, dir, topRelPath)
		dirRelLabels = append(dirRelLabels, bazel.Label{Label: "./" + dirRelPath})
	}
	// Return the package boudary resolved labels
	return TransformSubpackagePaths(ctx.Config(), dir, bazel.MakeLabelList(dirRelLabels))
}

// Returns true if a prefix + components[:i] is a package boundary.
//
// A package boundary is determined by a BUILD file in the directory. This can happen in 2 cases:
//
//  1. An Android.bp exists, which bp2build will always convert to a sibling BUILD file.
//  2. An Android.bp doesn't exist, but a checked-in BUILD/BUILD.bazel file exists, and that file
//     is allowlisted by the bp2build configuration to be merged into the symlink forest workspace.
func isPackageBoundary(config Config, prefix string, components []string, componentIndex int) bool {
	isSymlink := func(c Config, path string) bool {
		f, err := c.fs.Lstat(path)
		if err != nil {
			// The file does not exist
			return false
		}
		return f.Mode()&os.ModeSymlink == os.ModeSymlink
	}
	prefix = filepath.Join(prefix, filepath.Join(components[:componentIndex+1]...))
	if exists, _, _ := config.fs.Exists(filepath.Join(prefix, "Android.bp")); exists {
		return true
	} else if config.Bp2buildPackageConfig.ShouldKeepExistingBuildFileForDir(prefix) || isSymlink(config, prefix) {
		if exists, _, _ := config.fs.Exists(filepath.Join(prefix, "BUILD")); exists {
			return true
		} else if exists, _, _ := config.fs.Exists(filepath.Join(prefix, "BUILD.bazel")); exists {
			return true
		}
	}

	return false
}

// Transform a path (if necessary) to acknowledge package boundaries
//
// e.g. something like
//
//	async_safe/include/async_safe/CHECK.h
//
// might become
//
//	//bionic/libc/async_safe:include/async_safe/CHECK.h
//
// if the "async_safe" directory is actually a package and not just a directory.
//
// In particular, paths that extend into packages are transformed into absolute labels beginning with //.
func transformSubpackagePath(cfg Config, dir string, path bazel.Label) bazel.Label {
	var newPath bazel.Label

	// Don't transform OriginalModuleName
	newPath.OriginalModuleName = path.OriginalModuleName
	// if it wasn't a module, store the original path. We may need the original path to replace
	// references if it is actually in another package
	if path.OriginalModuleName == "" {
		newPath.OriginalModuleName = path.Label
	}

	if strings.HasPrefix(path.Label, "//") {
		// Assume absolute labels are already correct (e.g. //path/to/some/package:foo.h)
		newPath.Label = path.Label
		return newPath
	}
	if strings.HasPrefix(path.Label, "./") {
		// Drop "./" for consistent handling of paths.
		// Specifically, to not let "." be considered a package boundary.
		// Say `inputPath` is `x/Android.bp` and that file has some module
		// with `srcs=["y/a.c", "z/b.c"]`.
		// And say the directory tree is:
		//     x
		//     ├── Android.bp
		//     ├── y
		//     │   ├── a.c
		//     │   └── Android.bp
		//     └── z
		//         └── b.c
		// Then bazel equivalent labels in srcs should be:
		//   //x/y:a.c, x/z/b.c
		// The above should still be the case if `x/Android.bp` had
		//   srcs=["./y/a.c", "./z/b.c"]
		// However, if we didn't strip "./", we'd get
		//   //x/./y:a.c, //x/.:z/b.c
		path.Label = strings.TrimPrefix(path.Label, "./")
	}
	pathComponents := strings.Split(path.Label, "/")
	newLabel := ""
	foundPackageBoundary := false
	// Check the deepest subdirectory first and work upwards
	for i := len(pathComponents) - 1; i >= 0; i-- {
		pathComponent := pathComponents[i]
		var sep string
		if !foundPackageBoundary && isPackageBoundary(cfg, dir, pathComponents, i) {
			sep = ":"
			foundPackageBoundary = true
		} else {
			sep = "/"
		}
		if newLabel == "" {
			newLabel = pathComponent
		} else {
			newLabel = pathComponent + sep + newLabel
		}
	}
	if foundPackageBoundary {
		// Ensure paths end up looking like //bionic/... instead of //./bionic/...
		moduleDir := dir
		if strings.HasPrefix(moduleDir, ".") {
			moduleDir = moduleDir[1:]
		}
		// Make the path into an absolute label (e.g. //bionic/libc/foo:bar.h instead of just foo:bar.h)
		if moduleDir == "" {
			newLabel = "//" + newLabel
		} else {
			newLabel = "//" + moduleDir + "/" + newLabel
		}
	}
	newPath.Label = newLabel

	return newPath
}

// Transform paths to acknowledge package boundaries
// See transformSubpackagePath() for more information
func TransformSubpackagePaths(cfg Config, dir string, paths bazel.LabelList) bazel.LabelList {
	var newPaths bazel.LabelList
	for _, include := range paths.Includes {
		newPaths.Includes = append(newPaths.Includes, transformSubpackagePath(cfg, dir, include))
	}
	for _, exclude := range paths.Excludes {
		newPaths.Excludes = append(newPaths.Excludes, transformSubpackagePath(cfg, dir, exclude))
	}
	return newPaths
}

// Converts root-relative Paths to a list of bazel.Label relative to the module in ctx.
func RootToModuleRelativePaths(ctx BazelConversionPathContext, paths Paths) []bazel.Label {
	var newPaths []bazel.Label
	for _, path := range PathsWithModuleSrcSubDir(ctx, paths, "") {
		s := path.Rel()
		newPaths = append(newPaths, bazel.Label{Label: s})
	}
	return newPaths
}

var Bp2buildDepTag bp2buildDepTag

type bp2buildDepTag struct {
	blueprint.BaseDependencyTag
}

// expandSrcsForBazel returns bazel.LabelList with paths rooted from the module's local source
// directory and Bazel target labels, excluding those included in the excludes argument (which
// should already be expanded to resolve references to Soong-modules). Valid elements of paths
// include:
//   - filepath, relative to local module directory, resolves as a filepath relative to the local
//     source directory
//   - glob, relative to the local module directory, resolves as filepath(s), relative to the local
//     module directory. Because Soong does not have a concept of crossing package boundaries, the
//     glob as computed by Soong may contain paths that cross package-boundaries that would be
//     unknowingly omitted if the glob were handled by Bazel. To allow identification and detect
//     (within Bazel) use of paths that cross package boundaries, we expand globs within Soong rather
//     than converting Soong glob syntax to Bazel glob syntax. **Invalid for excludes.**
//   - other modules using the ":name{.tag}" syntax. These modules must implement SourceFileProducer
//     or OutputFileProducer. These resolve as a Bazel label for a target. If the Bazel target is in
//     the local module directory, it will be returned relative to the current package (e.g.
//     ":<target>"). Otherwise, it will be returned as an absolute Bazel label (e.g.
//     "//path/to/dir:<target>"). If the reference to another module cannot be resolved,the function
//     will panic.
//
// Properties passed as the paths or excludes argument must have been annotated with struct tag
// `android:"path"` so that dependencies on other modules will have already been handled by the
// pathdeps mutator.
func expandSrcsForBazel(ctx Bp2buildMutatorContext, paths, expandedExcludes []string, markAsDeps bool) bazel.LabelList {
	if paths == nil {
		return bazel.LabelList{}
	}
	labels := bazel.LabelList{
		Includes: []bazel.Label{},
	}

	// expandedExcludes contain module-dir relative paths, but root-relative paths
	// are needed for GlobFiles later.
	var rootRelativeExpandedExcludes []string
	for _, e := range expandedExcludes {
		rootRelativeExpandedExcludes = append(rootRelativeExpandedExcludes, filepath.Join(ctx.ModuleDir(), e))
	}

	for _, p := range paths {
		if m, tag := SrcIsModuleWithTag(p); m != "" {
			l := getOtherModuleLabel(ctx, m, tag, BazelModuleLabel, markAsDeps)
			if l != nil && !InList(l.Label, expandedExcludes) {
				if strings.HasPrefix(m, "//") {
					// this is a module in a soong namespace
					// It appears as //<namespace>:<module_name> in srcs, and not ://<namespace>:<module_name>
					l.OriginalModuleName = m
				} else {
					l.OriginalModuleName = fmt.Sprintf(":%s", m)
				}
				labels.Includes = append(labels.Includes, *l)
			}
		} else {
			var expandedPaths []bazel.Label
			if pathtools.IsGlob(p) {
				// e.g. turn "math/*.c" in
				// external/arm-optimized-routines to external/arm-optimized-routines/math/*.c
				rootRelativeGlobPath := pathForModuleSrc(ctx, p).String()
				expandedPaths = RootToModuleRelativePaths(ctx, GlobFiles(ctx, rootRelativeGlobPath, rootRelativeExpandedExcludes))
			} else {
				if !InList(p, expandedExcludes) {
					expandedPaths = append(expandedPaths, bazel.Label{Label: p})
				}
			}
			labels.Includes = append(labels.Includes, expandedPaths...)
		}
	}
	return labels
}

// getOtherModuleLabel returns a bazel.Label for the given dependency/tag combination for the
// module. The label will be relative to the current directory if appropriate. The dependency must
// already be resolved by either deps mutator or path deps mutator.
func getOtherModuleLabel(ctx Bp2buildMutatorContext, dep, tag string,
	labelFromModule func(BazelConversionPathContext, blueprint.Module) string,
	markAsDep bool) *bazel.Label {
	m, _ := ctx.ModuleFromName(dep)
	// The module was not found in an Android.bp file, this is often due to:
	//		* a limited manifest
	//		* a required module not being converted from Android.mk
	if m == nil {
		ctx.AddMissingBp2buildDep(dep)
		return &bazel.Label{
			Label: ":" + dep + "__BP2BUILD__MISSING__DEP",
		}
	}
	// Returns true if a dependency from the current module to the target module
	// should be skipped; doing so is a hack to circumvent certain problematic
	// scenarios that will be addressed in the future.
	shouldSkipDep := func(dep string) bool {
		// Don't count dependencies of "libc". This is a hack to circumvent the
		// fact that, in a variantless build graph, "libc" has a dependency on itself.
		if ctx.ModuleName() == "libc" {
			return true
		}

		// TODO: b/303307672: Dependencies on this module happen to "work" because
		// there is a source file with the same name as this module in the
		// same directory. We should remove this hack and enforce the underlying
		// module of this name is the actual one used.
		if dep == "mke2fs.conf" {
			return true
		}

		// TODO: b/303310285: Remove this special-casing once all dependencies of
		// crtbegin_dynamic are convertible
		if ctx.ModuleName() == "crtbegin_dynamic" {
			return true
		}

		return false
	}
	if markAsDep && !shouldSkipDep(dep) {
		ctx.AddDependency(ctx.Module(), Bp2buildDepTag, dep)
	}
	if !convertedToBazel(ctx, m) {
		ctx.AddUnconvertedBp2buildDep(dep)
	}
	label := BazelModuleLabel(ctx, ctx.Module())
	otherLabel := labelFromModule(ctx, m)

	// TODO(b/165114590): Convert tag (":name{.tag}") to corresponding Bazel implicit output targets.
	if (tag != "" && m.Name() == "framework-res") ||
		(tag == ".generated_srcjars" && ctx.OtherModuleType(m) == "java_aconfig_library") {
		otherLabel += tag
	}

	if samePackage(label, otherLabel) {
		otherLabel = bazelShortLabel(otherLabel)
	}

	return &bazel.Label{
		Label: otherLabel,
	}
}

func BazelModuleLabel(ctx BazelConversionPathContext, module blueprint.Module) string {
	// TODO(b/165114590): Convert tag (":name{.tag}") to corresponding Bazel implicit output targets.
	if !convertedToBazel(ctx, module) || isGoModule(module) {
		return bp2buildModuleLabel(ctx, module)
	}
	b, _ := module.(Bazelable)
	return b.GetBazelLabel(ctx, module)
}

func bazelShortLabel(label string) string {
	i := strings.Index(label, ":")
	if i == -1 {
		panic(fmt.Errorf("Could not find the ':' character in '%s', expected a fully qualified label.", label))
	}
	return label[i:]
}

func bazelPackage(label string) string {
	i := strings.Index(label, ":")
	if i == -1 {
		panic(fmt.Errorf("Could not find the ':' character in '%s', expected a fully qualified label.", label))
	}
	return label[0:i]
}

func samePackage(label1, label2 string) bool {
	return bazelPackage(label1) == bazelPackage(label2)
}

func bp2buildModuleLabel(ctx BazelConversionContext, module blueprint.Module) string {
	moduleName := moduleNameWithPossibleOverride(ctx, module, ctx.OtherModuleName(module))
	moduleDir := moduleDirWithPossibleOverride(ctx, module, ctx.OtherModuleDir(module))
	if moduleDir == Bp2BuildTopLevel {
		moduleDir = ""
	}
	if a, ok := module.(Module); ok && IsModulePrebuilt(a) {
		moduleName = RemoveOptionalPrebuiltPrefix(moduleName)
	}
	return fmt.Sprintf("//%s:%s", moduleDir, moduleName)
}

// BazelOutPath is a Bazel output path compatible to be used for mixed builds within Soong/Ninja.
type BazelOutPath struct {
	OutputPath
}

// ensure BazelOutPath implements Path
var _ Path = BazelOutPath{}

// ensure BazelOutPath implements genPathProvider
var _ genPathProvider = BazelOutPath{}

// ensure BazelOutPath implements objPathProvider
var _ objPathProvider = BazelOutPath{}

func (p BazelOutPath) genPathWithExt(ctx ModuleOutPathContext, subdir, ext string) ModuleGenPath {
	return PathForModuleGen(ctx, subdir, pathtools.ReplaceExtension(p.path, ext))
}

func (p BazelOutPath) objPathWithExt(ctx ModuleOutPathContext, subdir, ext string) ModuleObjPath {
	return PathForModuleObj(ctx, subdir, pathtools.ReplaceExtension(p.path, ext))
}

// PathForBazelOutRelative returns a BazelOutPath representing the path under an output directory dedicated to
// bazel-owned outputs. Calling .Rel() on the result will give the input path as relative to the given
// relativeRoot.
func PathForBazelOutRelative(ctx PathContext, relativeRoot string, path string) BazelOutPath {
	validatedPath, err := validatePath(filepath.Join("execroot", "__main__", path))
	if err != nil {
		reportPathError(ctx, err)
	}
	var relativeRootPath string
	if pathComponents := strings.SplitN(path, "/", 4); len(pathComponents) >= 3 &&
		pathComponents[0] == "bazel-out" && pathComponents[2] == "bin" {
		// If the path starts with something like: bazel-out/linux_x86_64-fastbuild-ST-b4ef1c4402f9/bin/
		// make it relative to that folder. bazel-out/volatile-status.txt is an example
		// of something that starts with bazel-out but is not relative to the bin folder
		relativeRootPath = filepath.Join("execroot", "__main__", pathComponents[0], pathComponents[1], pathComponents[2], relativeRoot)
	} else {
		relativeRootPath = filepath.Join("execroot", "__main__", relativeRoot)
	}

	var relPath string
	if relPath, err = filepath.Rel(relativeRootPath, validatedPath); err != nil || strings.HasPrefix(relPath, "../") {
		// We failed to make this path relative to execroot/__main__, fall back to a non-relative path
		// One case where this happens is when path is ../bazel_tools/something
		relativeRootPath = ""
		relPath = validatedPath
	}

	outputPath := OutputPath{
		basePath{"", ""},
		ctx.Config().soongOutDir,
		ctx.Config().BazelContext.OutputBase(),
	}

	return BazelOutPath{
		// .withRel() appends its argument onto the current path, and only the most
		// recently appended part is returned by outputPath.rel().
		// So outputPath.rel() will return relPath.
		OutputPath: outputPath.withRel(relativeRootPath).withRel(relPath),
	}
}

// PathForBazelOut returns a BazelOutPath representing the path under an output directory dedicated to
// bazel-owned outputs.
func PathForBazelOut(ctx PathContext, path string) BazelOutPath {
	return PathForBazelOutRelative(ctx, "", path)
}

// PathsForBazelOut returns a list of paths representing the paths under an output directory
// dedicated to Bazel-owned outputs.
func PathsForBazelOut(ctx PathContext, paths []string) Paths {
	outs := make(Paths, 0, len(paths))
	for _, p := range paths {
		outs = append(outs, PathForBazelOut(ctx, p))
	}
	return outs
}

// BazelStringOrLabelFromProp splits a Soong module property that can be
// either a string literal, path (with android:path tag) or a module reference
// into separate bazel string or label attributes. Bazel treats string and label
// attributes as distinct types, so this function categorizes a string property
// into either one of them.
//
// e.g. apex.private_key = "foo.pem" can either refer to:
//
// 1. "foo.pem" in the current directory -> file target
// 2. "foo.pem" module -> rule target
// 3. "foo.pem" file in a different directory, prefixed by a product variable handled
// in a bazel macro. -> string literal
//
// For the first two cases, they are defined using the label attribute. For the third case,
// it's defined with the string attribute.
func BazelStringOrLabelFromProp(
	ctx Bp2buildMutatorContext,
	propToDistinguish *string) (bazel.LabelAttribute, bazel.StringAttribute) {

	var labelAttr bazel.LabelAttribute
	var strAttr bazel.StringAttribute

	if propToDistinguish == nil {
		// nil pointer
		return labelAttr, strAttr
	}

	prop := String(propToDistinguish)
	if SrcIsModule(prop) != "" {
		// If it's a module (SrcIsModule will return the module name), set the
		// resolved label to the label attribute.
		labelAttr.SetValue(BazelLabelForModuleDepSingle(ctx, prop))
	} else {
		// Not a module name. This could be a string literal or a file target in
		// the current dir. Check if the path exists:
		path := ExistentPathForSource(ctx, ctx.ModuleDir(), prop)

		if path.Valid() && parentDir(path.String()) == ctx.ModuleDir() {
			// If it exists and the path is relative to the current dir, resolve the bazel label
			// for the _file target_ and set it to the label attribute.
			//
			// Resolution is necessary because this could be a file in a subpackage.
			labelAttr.SetValue(BazelLabelForModuleSrcSingle(ctx, prop))
		} else {
			// Otherwise, treat it as a string literal and assign to the string attribute.
			strAttr.Value = propToDistinguish
		}
	}

	return labelAttr, strAttr
}
