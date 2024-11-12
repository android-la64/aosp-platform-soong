// Copyright 2019 The Android Open Source Project
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

package config

import (
	"strings"

	"android/soong/android"
)

var (
	Loongarch64RustFlags            = []string{}
	Loongarch64ArchFeatureRustFlags = map[string][]string{}
	Loongarch64LinkFlags            = []string{
		"-Wl,--icf=safe",
		"-Wl,-z,max-page-size=16384",
		"-Wl,-z,separate-code",
	}

	//Loongarch64ArchVariantRustFlags = map[string][]string{
  //  "armv8-2a-dotprod":   []string{},  // XC-TODO: use loongarch64 cpu
  //}
)

func init() {
	registerToolchainFactory(android.Android, android.Loongarch64, Loongarch64ToolchainFactory)

	pctx.StaticVariable("Loongarch64ToolchainRustFlags", strings.Join(Loongarch64RustFlags, " "))
	pctx.StaticVariable("Loongarch64ToolchainLinkFlags", strings.Join(Loongarch64LinkFlags, " "))

	//for variant, rustFlags := range Loongarch64ArchVariantRustFlags {
	//	pctx.StaticVariable("Loongarch64"+variant+"VariantRustFlags",
	//		strings.Join(rustFlags, " "))
	//}

}

type toolchainLoongarch64 struct {
	toolchain64Bit
	toolchainRustFlags string
}

func (t *toolchainLoongarch64) RustTriple() string {
	return "loongarch64-unknown-none"
}

func (t *toolchainLoongarch64) ToolchainLinkFlags() string {
	return "${config.DeviceGlobalLinkFlags} ${config.Loongarch64ToolchainLinkFlags}"
}

func (t *toolchainLoongarch64) ToolchainRustFlags() string {
	return t.toolchainRustFlags
}

func (t *toolchainLoongarch64) RustFlags() string {
	return "${config.Loongarch64ToolchainRustFlags}"
}

func (t *toolchainLoongarch64) Supported() bool {
	return true
}

func (toolchainLoongarch64) LibclangRuntimeLibraryArch() string {
	return "loongarch64"
}

func Loongarch64ToolchainFactory(arch android.Arch) Toolchain {
	toolchainRustFlags := []string{
		"${config.Loongarch64ToolchainRustFlags}",
		//"${config.Loongarch64" + arch.ArchVariant + "VariantRustFlags}",
	}

	toolchainRustFlags = append(toolchainRustFlags, deviceGlobalRustFlags...)

	for _, feature := range arch.ArchFeatures {
		toolchainRustFlags = append(toolchainRustFlags, Loongarch64ArchFeatureRustFlags[feature]...)
	}

	return &toolchainLoongarch64{
		toolchainRustFlags: strings.Join(toolchainRustFlags, " "),
	}
}
