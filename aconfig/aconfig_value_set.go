// Copyright 2023 Google Inc. All rights reserved.
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

package aconfig

import (
	"android/soong/android"
	"android/soong/bazel"
	"github.com/google/blueprint"
)

// Properties for "aconfig_value_set"
type ValueSetModule struct {
	android.ModuleBase
	android.DefaultableModuleBase
	android.BazelModuleBase

	properties struct {
		// aconfig_values modules
		Values []string
	}
}

func ValueSetFactory() android.Module {
	module := &ValueSetModule{}

	android.InitAndroidModule(module)
	android.InitDefaultableModule(module)
	module.AddProperties(&module.properties)
	android.InitBazelModule(module)

	return module
}

// Dependency tag for values property
type valueSetType struct {
	blueprint.BaseDependencyTag
}

var valueSetTag = valueSetType{}

// Provider published by aconfig_value_set
type valueSetProviderData struct {
	// The package of each of the
	// (map of package --> aconfig_module)
	AvailablePackages map[string]android.Paths
}

var valueSetProviderKey = blueprint.NewProvider(valueSetProviderData{})

func (module *ValueSetModule) DepsMutator(ctx android.BottomUpMutatorContext) {
	deps := ctx.AddDependency(ctx.Module(), valueSetTag, module.properties.Values...)
	for _, dep := range deps {
		_, ok := dep.(*ValuesModule)
		if !ok {
			ctx.PropertyErrorf("values", "values must be a aconfig_values module")
			return
		}
	}
}

func (module *ValueSetModule) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	// Accumulate the packages of the values modules listed, and set that as an
	// valueSetProviderKey provider that aconfig modules can read and use
	// to append values to their aconfig actions.
	packages := make(map[string]android.Paths)
	ctx.VisitDirectDeps(func(dep android.Module) {
		if !ctx.OtherModuleHasProvider(dep, valuesProviderKey) {
			// Other modules get injected as dependencies too, for example the license modules
			return
		}
		depData := ctx.OtherModuleProvider(dep, valuesProviderKey).(valuesProviderData)

		srcs := make([]android.Path, len(depData.Values))
		copy(srcs, depData.Values)
		packages[depData.Package] = srcs

	})
	ctx.SetProvider(valueSetProviderKey, valueSetProviderData{
		AvailablePackages: packages,
	})
}

type bazelAconfigValueSetAttributes struct {
	Values bazel.LabelListAttribute
}

func (module *ValueSetModule) ConvertWithBp2build(ctx android.Bp2buildMutatorContext) {
	if ctx.ModuleType() != "aconfig_value_set" {
		return
	}

	attrs := bazelAconfigValueSetAttributes{
		Values: bazel.MakeLabelListAttribute(android.BazelLabelForModuleDeps(ctx, module.properties.Values)),
	}
	props := bazel.BazelTargetModuleProperties{
		Rule_class:        "aconfig_value_set",
		Bzl_load_location: "//build/bazel/rules/aconfig:aconfig_value_set.bzl",
	}

	ctx.CreateBazelTargetModule(props, android.CommonAttributes{Name: module.Name()}, &attrs)
}
