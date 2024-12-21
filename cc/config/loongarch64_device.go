// Copyright 2022 Google Inc. All rights reserved.
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
	"fmt"
	"strings"

	"android/soong/android"
)

var (
	loongarch64Cflags = []string{
		// Help catch common 32/64-bit errors. (This is duplicated in all 64-bit
		// architectures' cflags.)
		"-Werror=implicit-function-declaration",
	}

	loongarch64ArchVariantCflags = map[string][]string{}

	loongarch64Ldflags = []string{
	}

	loongarch64Lldflags = append(loongarch64Ldflags,
		"-Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=4096",
	)

	loongarch64Cppflags = []string{}

	loongarch64CpuVariantCflags = map[string][]string{}
)

const ()

func init() {

	pctx.StaticVariable("Loongarch64Ldflags", strings.Join(loongarch64Ldflags, " "))
	pctx.StaticVariable("Loongarch64Lldflags", strings.Join(loongarch64Lldflags, " "))

	pctx.StaticVariable("Loongarch64Cflags", strings.Join(loongarch64Cflags, " "))
	pctx.StaticVariable("Loongarch64Cppflags", strings.Join(loongarch64Cppflags, " "))
}

var (
	loongarch64ArchVariantCflagsVar = map[string]string{}

	loongarch64CpuVariantCflagsVar = map[string]string{}

	loongarch64CpuVariantLdflags = map[string]string{}
)

type toolchainLoongarch64 struct {
	toolchainBionic
	toolchain64Bit

	ldflags         string
	lldflags        string
	toolchainCflags string
}

func (t *toolchainLoongarch64) Name() string {
	return "loongarch64"
}

func (t *toolchainLoongarch64) IncludeFlags() string {
	return ""
}

func (t *toolchainLoongarch64) ClangTriple() string {
	return "loongarch64-linux-android"
}

func (t *toolchainLoongarch64) Cflags() string {
	return "${config.Loongarch64Cflags}"
}

func (t *toolchainLoongarch64) Cppflags() string {
	return "${config.Loongarch64Cppflags}"
}

func (t *toolchainLoongarch64) Ldflags() string {
	return t.ldflags
}

func (t *toolchainLoongarch64) Lldflags() string {
	return t.lldflags
}

func (t *toolchainLoongarch64) ToolchainCflags() string {
	return t.toolchainCflags
}

func (toolchainLoongarch64) LibclangRuntimeLibraryArch() string {
	return "loongarch64"
}

func loongarch64ToolchainFactory(arch android.Arch) Toolchain {
	switch arch.ArchVariant {
	case "":
	default:
		panic(fmt.Sprintf("Unknown Loongarch64 architecture version: %q", arch.ArchVariant))
	}

	toolchainCflags := []string{loongarch64ArchVariantCflagsVar[arch.ArchVariant]}
	toolchainCflags = append(toolchainCflags,
		variantOrDefault(loongarch64CpuVariantCflagsVar, arch.CpuVariant))

	extraLdflags := variantOrDefault(loongarch64CpuVariantLdflags, arch.CpuVariant)
	return &toolchainLoongarch64{
		ldflags: strings.Join([]string{
			"${config.Loongarch64Ldflags}",
			extraLdflags,
		}, " "),
		lldflags: strings.Join([]string{
			"${config.Loongarch64Lldflags}",
			extraLdflags,
		}, " "),
		toolchainCflags: strings.Join(toolchainCflags, " "),
	}
}

func init() {
	registerToolchainFactory(android.Android, android.Loongarch64, loongarch64ToolchainFactory)
}
