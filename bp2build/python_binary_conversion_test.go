package bp2build

import (
	"testing"

	"android/soong/android"
	"android/soong/genrule"
	"android/soong/python"
)

func runBp2BuildTestCaseWithPythonLibraries(t *testing.T, tc Bp2buildTestCase) {
	t.Helper()
	RunBp2BuildTestCase(t, func(ctx android.RegistrationContext) {
		ctx.RegisterModuleType("python_library", python.PythonLibraryFactory)
		ctx.RegisterModuleType("python_library_host", python.PythonLibraryHostFactory)
		ctx.RegisterModuleType("genrule", genrule.GenRuleFactory)
		ctx.RegisterModuleType("python_defaults", python.DefaultsFactory)
	}, tc)
}

func TestPythonBinaryHostSimple(t *testing.T) {
	runBp2BuildTestCaseWithPythonLibraries(t, Bp2buildTestCase{
		Description:                "simple python_binary_host converts to a native py_binary",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Filesystem: map[string]string{
			"a.py":           "",
			"b/c.py":         "",
			"b/d.py":         "",
			"b/e.py":         "",
			"files/data.txt": "",
		},
		StubbedBuildDefinitions: []string{"bar"},
		Blueprint: `python_binary_host {
    name: "foo",
    main: "a.py",
    srcs: ["**/*.py"],
    exclude_srcs: ["b/e.py"],
    data: ["files/data.txt",],
    libs: ["bar"],
    bazel_module: { bp2build_available: true },
}
    python_library_host {
      name: "bar",
      srcs: ["b/e.py"],
    }`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"data":    `["files/data.txt"]`,
				"deps":    `[":bar"]`,
				"main":    `"a.py"`,
				"imports": `["."]`,
				"srcs": `[
        "a.py",
        "b/c.py",
        "b/d.py",
    ]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryHostPy2(t *testing.T) {
	RunBp2BuildTestCaseSimple(t, Bp2buildTestCase{
		Description:                "py2 python_binary_host",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Blueprint: `python_binary_host {
    name: "foo",
    srcs: ["a.py"],
    version: {
        py2: {
            enabled: true,
        },
        py3: {
            enabled: false,
        },
    },

    bazel_module: { bp2build_available: true },
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"python_version": `"PY2"`,
				"imports":        `["."]`,
				"srcs":           `["a.py"]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryHostPy3(t *testing.T) {
	RunBp2BuildTestCaseSimple(t, Bp2buildTestCase{
		Description:                "py3 python_binary_host",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Blueprint: `python_binary_host {
    name: "foo",
    srcs: ["a.py"],
    version: {
        py2: {
            enabled: false,
        },
        py3: {
            enabled: true,
        },
    },

    bazel_module: { bp2build_available: true },
}
`,
		ExpectedBazelTargets: []string{
			// python_version is PY3 by default.
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"imports": `["."]`,
				"srcs":    `["a.py"]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryHostArchVariance(t *testing.T) {
	RunBp2BuildTestCaseSimple(t, Bp2buildTestCase{
		Description:                "test arch variants",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Filesystem: map[string]string{
			"dir/arm.py": "",
			"dir/x86.py": "",
		},
		Blueprint: `python_binary_host {
					 name: "foo-arm",
					 arch: {
						 arm: {
							 srcs: ["arm.py"],
						 },
						 x86: {
							 srcs: ["x86.py"],
						 },
					},
				 }`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo-arm", AttrNameToString{
				"imports": `["."]`,
				"srcs": `select({
        "//build/bazel_common_rules/platforms/arch:arm": ["arm.py"],
        "//build/bazel_common_rules/platforms/arch:x86": ["x86.py"],
        "//conditions:default": [],
    })`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryMainIsNotSpecified(t *testing.T) {
	runBp2BuildTestCaseWithPythonLibraries(t, Bp2buildTestCase{
		Description:                "python_binary_host main label in same package",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Blueprint: `python_binary_host {
    name: "foo",
    bazel_module: { bp2build_available: true },
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"imports": `["."]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryMainIsLabel(t *testing.T) {
	runBp2BuildTestCaseWithPythonLibraries(t, Bp2buildTestCase{
		Description:                "python_binary_host main label in same package",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		StubbedBuildDefinitions:    []string{"a"},
		Blueprint: `python_binary_host {
    name: "foo",
    main: ":a",
    bazel_module: { bp2build_available: true },
}

genrule {
		name: "a",
}
`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"main":    `":a"`,
				"imports": `["."]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryMainIsSubpackageFile(t *testing.T) {
	runBp2BuildTestCaseWithPythonLibraries(t, Bp2buildTestCase{
		Description:                "python_binary_host main is subpackage file",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Filesystem: map[string]string{
			"a/Android.bp": "",
			"a/b.py":       "",
		},
		Blueprint: `python_binary_host {
    name: "foo",
    main: "a/b.py",
    bazel_module: { bp2build_available: true },
}

`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"main":    `"//a:b.py"`,
				"imports": `["."]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryMainIsSubDirFile(t *testing.T) {
	runBp2BuildTestCaseWithPythonLibraries(t, Bp2buildTestCase{
		Description:                "python_binary_host main is file in sub directory that is not Bazel package",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		Filesystem: map[string]string{
			"a/b.py": "",
		},
		Blueprint: `python_binary_host {
    name: "foo",
    main: "a/b.py",
    bazel_module: { bp2build_available: true },
}

`,
		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"main":    `"a/b.py"`,
				"imports": `["."]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}

func TestPythonBinaryDuplicatesInRequired(t *testing.T) {
	runBp2BuildTestCaseWithPythonLibraries(t, Bp2buildTestCase{
		Description:                "python_binary_host duplicates in required attribute of the module and its defaults",
		ModuleTypeUnderTest:        "python_binary_host",
		ModuleTypeUnderTestFactory: python.PythonBinaryHostFactory,
		StubbedBuildDefinitions:    []string{"r1", "r2"},
		Blueprint: `python_binary_host {
    name: "foo",
    main: "a.py",
		defaults: ["d"],
    required: [
        "r1",
    ],
    bazel_module: { bp2build_available: true },
}

python_defaults {
    name: "d",
    required: [
        "r1",
        "r2",
    ],
}` + simpleModule("genrule", "r1") +
			simpleModule("genrule", "r2"),

		ExpectedBazelTargets: []string{
			MakeBazelTarget("py_binary", "foo", AttrNameToString{
				"main":    `"a.py"`,
				"imports": `["."]`,
				"data": `[
        ":r1",
        ":r2",
    ]`,
				"target_compatible_with": `select({
        "//build/bazel_common_rules/platforms/os:android": ["@platforms//:incompatible"],
        "//conditions:default": [],
    })`,
			}),
		},
	})
}
