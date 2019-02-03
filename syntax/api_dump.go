// Copyright (c) 2019, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"go/types"
	"os"
	"reflect"

	"golang.org/x/tools/go/packages"
)

type Package struct {
	Types map[string]NamedType `json:"types"`
	Funcs map[string]DocType   `json:"funcs"`
}

type NamedType struct {
	Doc  string      `json:"doc"`
	Type interface{} `json:"type"`

	Methods map[string]DocType `json:"methods"`
}

type DocType struct {
	Doc  string      `json:"doc"`
	Type interface{} `json:"type"`
}

func main() {
	cfg := &packages.Config{Mode: packages.LoadSyntax}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}
	if len(pkgs) != 1 {
		panic("expected exactly one package")
	}

	dump := &Package{
		Types: map[string]NamedType{},
		Funcs: map[string]DocType{},
	}

	pkg := pkgs[0]
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		if fn, ok := obj.(*types.Func); ok {
			dump.Funcs[fn.Name()] = DocType{
				Type: dumpType(fn.Type()),
			}
			continue
		}
		tname, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		name := tname.Name()
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}

		dumpNamed := NamedType{
			Type:    dumpType(named.Underlying()),
			Methods: map[string]DocType{},
		}
		for i := 0; i < named.NumMethods(); i++ {
			fn := named.Method(i)
			if !fn.Exported() {
				continue
			}
			dumpNamed.Methods[fn.Name()] = DocType{
				Type: dumpType(fn.Type()),
			}
		}
		dump.Types[name] = dumpNamed
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")
	if err := enc.Encode(dump); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func dumpType(typ types.Type) interface{} {
	dump := map[string]interface{}{}
	switch typ := typ.(type) {
	case *types.Interface:
		dump["kind"] = "interface"
		methods := map[string]DocType{}
		for i := 0; i < typ.NumMethods(); i++ {
			fn := typ.Method(i)
			if !fn.Exported() {
				continue
			}
			methods[fn.Name()] = DocType{
				Type: dumpType(fn.Type()),
			}
		}
		dump["methods"] = methods
		return dump
	case *types.Struct:
		dump["kind"] = "struct"
		fields := map[string]DocType{}
		for i := 0; i < typ.NumFields(); i++ {
			fd := typ.Field(i)
			if !fd.Exported() {
				continue
			}
			fields[fd.Name()] = DocType{
				Type: dumpType(fd.Type()),
			}
		}
		dump["fields"] = fields
		return dump
	case *types.Slice:
		dump["kind"] = "list"
		dump["elem"] = dumpType(typ.Elem())
		return dump
	case *types.Pointer:
		dump["kind"] = "pointer"
		dump["elem"] = dumpType(typ.Elem())
		return dump
	case *types.Signature:
		dump["kind"] = "function"
		dump["params"] = dumpTuple(typ.Params())
		dump["results"] = dumpTuple(typ.Results())
		return dump
	case *types.Basic:
		return typ.String()
	case *types.Named:
		return typ.String()
	}
	panic("TODO: " + reflect.TypeOf(typ).String())
}

func dumpTuple(tuple *types.Tuple) []interface{} {
	typs := make([]interface{}, 0)
	for i := 0; i < tuple.Len(); i++ {
		vr := tuple.At(i)
		typs = append(typs, map[string]interface{}{
			"name": vr.Name(),
			"type": dumpType(vr.Type()),
		})
	}
	return typs
}
