// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mk2rbc

import (
	"fmt"
	"strings"
)

type variable interface {
	name() string
	emitGet(gctx *generationContext, isDefined bool)
	emitSet(gctx *generationContext, asgn *assignmentNode)
	emitDefined(gctx *generationContext)
	valueType() starlarkType
	setValueType(t starlarkType)
	defaultValueString() string
	isPreset() bool
}

type baseVariable struct {
	nam    string
	typ    starlarkType
	preset bool // true if it has been initialized at startup
}

func (v baseVariable) name() string {
	return v.nam
}

func (v baseVariable) valueType() starlarkType {
	return v.typ
}

func (v *baseVariable) setValueType(t starlarkType) {
	v.typ = t
}

func (v baseVariable) isPreset() bool {
	return v.preset
}

var defaultValuesByType = map[starlarkType]string{
	starlarkTypeUnknown: `""`,
	starlarkTypeList:    "[]",
	starlarkTypeString:  `""`,
	starlarkTypeInt:     "0",
	starlarkTypeBool:    "False",
	starlarkTypeVoid:    "None",
}

func (v baseVariable) defaultValueString() string {
	if v, ok := defaultValuesByType[v.valueType()]; ok {
		return v
	}
	panic(fmt.Errorf("%s has unknown type %q", v.name(), v.valueType()))
}

type productConfigVariable struct {
	baseVariable
}

func (pcv productConfigVariable) emitSet(gctx *generationContext, asgn *assignmentNode) {
	emitAssignment := func() {
		pcv.emitGet(gctx, true)
		gctx.write(" = ")
		asgn.value.emitListVarCopy(gctx)
	}
	emitAppend := func() {
		pcv.emitGet(gctx, true)
		gctx.write(" += ")
		value := asgn.value
		if pcv.valueType() == starlarkTypeString {
			gctx.writef(`" " + `)
			value = &toStringExpr{expr: value}
		}
		value.emit(gctx)
	}

	switch asgn.flavor {
	case asgnSet:
		emitAssignment()
	case asgnAppend:
		emitAppend()
	case asgnMaybeAppend:
		// If we are not sure variable has been assigned before, emit setdefault
		if pcv.typ == starlarkTypeList {
			gctx.writef("%s(handle, %q)", cfnSetListDefault, pcv.name())
		} else {
			gctx.writef("cfg.setdefault(%q, %s)", pcv.name(), pcv.defaultValueString())
		}
		gctx.newLine()
		emitAppend()
	case asgnMaybeSet:
		gctx.writef("if cfg.get(%q) == None:", pcv.nam)
		gctx.indentLevel++
		gctx.newLine()
		emitAssignment()
		gctx.indentLevel--
	}
}

func (pcv productConfigVariable) emitGet(gctx *generationContext, isDefined bool) {
	if isDefined || pcv.isPreset() {
		gctx.writef("cfg[%q]", pcv.nam)
	} else {
		gctx.writef("cfg.get(%q, %s)", pcv.nam, pcv.defaultValueString())
	}
}

func (pcv productConfigVariable) emitDefined(gctx *generationContext) {
	gctx.writef("g.get(%q) != None", pcv.name())
}

type otherGlobalVariable struct {
	baseVariable
}

func (scv otherGlobalVariable) emitSet(gctx *generationContext, asgn *assignmentNode) {
	emitAssignment := func() {
		scv.emitGet(gctx, true)
		gctx.write(" = ")
		asgn.value.emitListVarCopy(gctx)
	}

	emitAppend := func() {
		scv.emitGet(gctx, true)
		gctx.write(" += ")
		value := asgn.value
		if scv.valueType() == starlarkTypeString {
			gctx.writef(`" " + `)
			value = &toStringExpr{expr: value}
		}
		value.emit(gctx)
	}

	switch asgn.flavor {
	case asgnSet:
		emitAssignment()
	case asgnAppend:
		emitAppend()
	case asgnMaybeAppend:
		// If we are not sure variable has been assigned before, emit setdefault
		gctx.writef("g.setdefault(%q, %s)", scv.name(), scv.defaultValueString())
		gctx.newLine()
		emitAppend()
	case asgnMaybeSet:
		gctx.writef("if g.get(%q) == None:", scv.nam)
		gctx.indentLevel++
		gctx.newLine()
		emitAssignment()
		gctx.indentLevel--
	}
}

func (scv otherGlobalVariable) emitGet(gctx *generationContext, isDefined bool) {
	if isDefined || scv.isPreset() {
		gctx.writef("g[%q]", scv.nam)
	} else {
		gctx.writef("g.get(%q, %s)", scv.nam, scv.defaultValueString())
	}
}

func (scv otherGlobalVariable) emitDefined(gctx *generationContext) {
	gctx.writef("g.get(%q) != None", scv.name())
}

type localVariable struct {
	baseVariable
}

func (lv localVariable) emitDefined(gctx *generationContext) {
	gctx.writef(lv.String())
}

func (lv localVariable) String() string {
	return "_" + lv.nam
}

func (lv localVariable) emitSet(gctx *generationContext, asgn *assignmentNode) {
	switch asgn.flavor {
	case asgnSet:
		gctx.writef("%s = ", lv)
		asgn.value.emitListVarCopy(gctx)
	case asgnAppend:
		lv.emitGet(gctx, false)
		gctx.write(" += ")
		value := asgn.value
		if lv.valueType() == starlarkTypeString {
			gctx.writef(`" " + `)
			value = &toStringExpr{expr: value}
		}
		value.emit(gctx)
	case asgnMaybeAppend:
		gctx.writef("%s(%q, ", cfnLocalAppend, lv)
		asgn.value.emit(gctx)
		gctx.write(")")
	case asgnMaybeSet:
		gctx.writef("%s(%q, ", cfnLocalSetDefault, lv)
		asgn.value.emit(gctx)
		gctx.write(")")
	}
}

func (lv localVariable) emitGet(gctx *generationContext, _ bool) {
	gctx.writef("%s", lv)
}

type predefinedVariable struct {
	baseVariable
	value starlarkExpr
}

func (pv predefinedVariable) emitGet(gctx *generationContext, _ bool) {
	pv.value.emit(gctx)
}

func (pv predefinedVariable) emitSet(gctx *generationContext, asgn *assignmentNode) {
	if expectedValue, ok1 := maybeString(pv.value); ok1 {
		actualValue, ok2 := maybeString(asgn.value)
		if ok2 {
			if actualValue == expectedValue {
				return
			}
			gctx.emitConversionError(asgn.location,
				fmt.Sprintf("cannot set predefined variable %s to %q, its value should be %q",
					pv.name(), actualValue, expectedValue))
			gctx.starScript.hasErrors = true
			return
		}
	}
	panic(fmt.Errorf("cannot set predefined variable %s to %q", pv.name(), asgn.mkValue.Dump()))
}

func (pv predefinedVariable) emitDefined(gctx *generationContext) {
	gctx.write("True")
}

var localProductConfigVariables = map[string]string{
	"LOCAL_AUDIO_PRODUCT_PACKAGE":         "PRODUCT_PACKAGES",
	"LOCAL_AUDIO_PRODUCT_COPY_FILES":      "PRODUCT_COPY_FILES",
	"LOCAL_AUDIO_DEVICE_PACKAGE_OVERLAYS": "DEVICE_PACKAGE_OVERLAYS",
	"LOCAL_DUMPSTATE_PRODUCT_PACKAGE":     "PRODUCT_PACKAGES",
	"LOCAL_GATEKEEPER_PRODUCT_PACKAGE":    "PRODUCT_PACKAGES",
	"LOCAL_HEALTH_PRODUCT_PACKAGE":        "PRODUCT_PACKAGES",
	"LOCAL_SENSOR_PRODUCT_PACKAGE":        "PRODUCT_PACKAGES",
	"LOCAL_KEYMASTER_PRODUCT_PACKAGE":     "PRODUCT_PACKAGES",
	"LOCAL_KEYMINT_PRODUCT_PACKAGE":       "PRODUCT_PACKAGES",
}

var presetVariables = map[string]bool{
	"BUILD_ID":                  true,
	"HOST_ARCH":                 true,
	"HOST_OS":                   true,
	"HOST_BUILD_TYPE":           true,
	"OUT_DIR":                   true,
	"PLATFORM_VERSION_CODENAME": true,
	"PLATFORM_VERSION":          true,
	"TARGET_ARCH":               true,
	"TARGET_ARCH_VARIANT":       true,
	"TARGET_BUILD_TYPE":         true,
	"TARGET_BUILD_VARIANT":      true,
	"TARGET_PRODUCT":            true,
}

// addVariable returns a variable with a given name. A variable is
// added if it does not exist yet.
func (ctx *parseContext) addVariable(name string) variable {
	v, found := ctx.variables[name]
	if !found {
		_, preset := presetVariables[name]
		if vi, found := KnownVariables[name]; found {
			switch vi.class {
			case VarClassConfig:
				v = &productConfigVariable{baseVariable{nam: name, typ: vi.valueType, preset: preset}}
			case VarClassSoong:
				v = &otherGlobalVariable{baseVariable{nam: name, typ: vi.valueType, preset: preset}}
			}
		} else if name == strings.ToLower(name) {
			// Heuristics: if variable's name is all lowercase, consider it local
			// string variable.
			v = &localVariable{baseVariable{nam: name, typ: starlarkTypeUnknown}}
		} else {
			vt := starlarkTypeUnknown
			if strings.HasPrefix(name, "LOCAL_") {
				// Heuristics: local variables that contribute to corresponding config variables
				if cfgVarName, found := localProductConfigVariables[name]; found {
					vi, found2 := KnownVariables[cfgVarName]
					if !found2 {
						panic(fmt.Errorf("unknown config variable %s for %s", cfgVarName, name))
					}
					vt = vi.valueType
				}
			}
			if strings.HasSuffix(name, "_LIST") && vt == starlarkTypeUnknown {
				// Heuristics: Variables with "_LIST" suffix are lists
				vt = starlarkTypeList
			}
			v = &otherGlobalVariable{baseVariable{nam: name, typ: vt}}
		}
		ctx.variables[name] = v
	}
	return v
}
