// Copyright (C) 2019 The Android Open Source Project
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
	"android/soong/android"
	"android/soong/bazel"
)

type SyspropLibraryLabels struct {
	SyspropLibraryLabel  string
	CcSharedLibraryLabel string
	CcStaticLibraryLabel string
	JavaLibraryLabel     string
}

// TODO(b/240463568): Additional properties will be added for API validation
type bazelSyspropLibraryAttributes struct {
	Srcs bazel.LabelListAttribute
	Tags bazel.StringListAttribute
}

func Bp2buildBaseSyspropLibrary(ctx android.Bp2buildMutatorContext, name string, srcs bazel.LabelListAttribute) {
	apexAvailableTags := android.ApexAvailableTagsWithoutTestApexes(ctx.(android.Bp2buildMutatorContext), ctx.Module())

	ctx.CreateBazelTargetModule(
		bazel.BazelTargetModuleProperties{
			Rule_class:        "sysprop_library",
			Bzl_load_location: "//build/bazel/rules/sysprop:sysprop_library.bzl",
		},
		android.CommonAttributes{Name: name},
		&bazelSyspropLibraryAttributes{
			Srcs: srcs,
			Tags: apexAvailableTags,
		},
	)
}

type bazelCcSyspropLibraryAttributes struct {
	Dep             bazel.LabelAttribute
	Min_sdk_version *string
	Tags            bazel.StringListAttribute
}

func Bp2buildSyspropCc(ctx android.Bp2buildMutatorContext, labels SyspropLibraryLabels, minSdkVersion *string) {
	apexAvailableTags := android.ApexAvailableTagsWithoutTestApexes(ctx.(android.Bp2buildMutatorContext), ctx.Module())

	attrs := &bazelCcSyspropLibraryAttributes{
		Dep:             *bazel.MakeLabelAttribute(":" + labels.SyspropLibraryLabel),
		Min_sdk_version: minSdkVersion,
		Tags:            apexAvailableTags,
	}

	if labels.CcSharedLibraryLabel != "" {
		ctx.CreateBazelTargetModule(
			bazel.BazelTargetModuleProperties{
				Rule_class:        "cc_sysprop_library_shared",
				Bzl_load_location: "//build/bazel/rules/cc:cc_sysprop_library.bzl",
			},
			android.CommonAttributes{Name: labels.CcSharedLibraryLabel},
			attrs)
	}
	if labels.CcStaticLibraryLabel != "" {
		ctx.CreateBazelTargetModule(
			bazel.BazelTargetModuleProperties{
				Rule_class:        "cc_sysprop_library_static",
				Bzl_load_location: "//build/bazel/rules/cc:cc_sysprop_library.bzl",
			},
			android.CommonAttributes{Name: labels.CcStaticLibraryLabel},
			attrs)
	}
}

type bazelJavaLibraryAttributes struct {
	Dep             bazel.LabelAttribute
	Min_sdk_version *string
	Tags            bazel.StringListAttribute
}

func Bp2buildSyspropJava(ctx android.Bp2buildMutatorContext, labels SyspropLibraryLabels, minSdkVersion *string) {
	apexAvailableTags := android.ApexAvailableTagsWithoutTestApexes(ctx.(android.Bp2buildMutatorContext), ctx.Module())

	ctx.CreateBazelTargetModule(
		bazel.BazelTargetModuleProperties{
			Rule_class:        "java_sysprop_library",
			Bzl_load_location: "//build/bazel/rules/java:java_sysprop_library.bzl",
		},
		android.CommonAttributes{Name: labels.JavaLibraryLabel},
		&bazelJavaLibraryAttributes{
			Dep:             *bazel.MakeLabelAttribute(":" + labels.SyspropLibraryLabel),
			Min_sdk_version: minSdkVersion,
			Tags:            apexAvailableTags,
		})
}
