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

package config

import (
	// "fmt"
	"strings"

	"android/soong/android"
)

var (
	loongarch64Cflags = []string{
		// Help catch common 32/64-bit errors.
		"-Werror=implicit-function-declaration",
		"-Wno-implicit-int-float-conversion",
		"-Wno-deprecated-copy",
	}

	loongarch64ClangCflags = append(loongarch64Cflags, []string{
		"-fintegrated-as",
		"-Wno-implicit-fallthrough",
	}...)

	loongarch64Cppflags = []string{
		"-Wno-implicit-int-float-conversion",
	}

	loongarch64Ldflags = []string{
		"-Wl,--allow-shlib-undefined",
	}

	loongarch64Lldflags = []string{ }

	// XC-TODO: more Variants?
	//loongarch64ClangCpuVariantCflags = map[string][]string{
	//}
)

const (
	loongarch64GccVersion = "8.4"
)

func init() {
	pctx.StaticVariable("loongarch64GccVersion", loongarch64GccVersion)

	pctx.SourcePathVariable("Loongarch64GccRoot",
		"prebuilts/gcc/${HostPrebuiltTag}/loongarch64/loongarch64-linux-android-${loongarch64GccVersion}")

	// Clang cflags
	pctx.StaticVariable("Loongarch64ClangCflags", strings.Join(ClangFilterUnknownCflags(loongarch64Cflags), " "))
	pctx.StaticVariable("Loongarch64ClangCppflags", strings.Join(ClangFilterUnknownCflags(loongarch64Cppflags), " "))
	pctx.StaticVariable("Loongarch64ClangLdflags", strings.Join(ClangFilterUnknownCflags(loongarch64Ldflags), " "))
	pctx.StaticVariable("Loongarch64ClangLldflags", strings.Join(ClangFilterUnknownCflags(loongarch64Lldflags), " "))

	// XC-TODO: variant
}

type toolchainLoongarch64 struct {
	toolchain64Bit

	clangCflags          string
	toolchainClangCflags string
}

func (t *toolchainLoongarch64) Name() string {
	return "loongarch64"
}

func (t *toolchainLoongarch64) GccRoot() string {
	return "${config.Loongarch64GccRoot}"
}

func (t *toolchainLoongarch64) GccTriple() string {
	return "loongarch64-linux-android"
}

func (t *toolchainLoongarch64) GccVersion() string {
	return loongarch64GccVersion
}

func (t *toolchainLoongarch64) IncludeFlags() string {
	return ""
}

func (t *toolchainLoongarch64) ClangTriple() string {
	return t.GccTriple()
}

func (t *toolchainLoongarch64) ClangCflags() string {
	return "${config.Loongarch64ClangCflags}"
}

func (t *toolchainLoongarch64) ClangCppflags() string {
	return "${config.Loongarch64ClangCppflags}"
}

func (t *toolchainLoongarch64) ClangLdflags() string {
	return "${config.Loongarch64ClangLdflags}"
}

func (t *toolchainLoongarch64) ClangLldflags() string {
	return "${config.Loongarch64ClangLldflags}"
}

func (t *toolchainLoongarch64) ToolchainClangCflags() string {
	return t.toolchainClangCflags
}

func (toolchainLoongarch64) LibclangRuntimeLibraryArch() string {
	return "loongarch64"
}

func loongarch64ToolchainFactory(arch android.Arch) Toolchain {
	return &toolchainLoongarch64{
		clangCflags:          "${config.Loongarch64ClangCflags}",
		toolchainClangCflags: "${config.Loongarch64ClangCflags}",
	}
}

func init() {
	registerToolchainFactory(android.Android, android.Loongarch64, loongarch64ToolchainFactory)
}
