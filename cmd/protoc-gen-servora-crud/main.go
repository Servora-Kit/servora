// Command protoc-gen-servora-crud generates runtime-independent CRUD companions.
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	var target string
	flags := flag.NewFlagSet("protoc-gen-servora-crud", flag.ContinueOnError)
	flags.StringVar(&target, "target", "go", "generation target: go or ts")
	protogen.Options{ParamFunc: flags.Set}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return generate(gen, target)
	})
}

func generate(gen *protogen.Plugin, target string) error {
	resourcesByFile := make(map[*protogen.File][]*resourceInfo)
	resourcesByName := make(map[string]*resourceInfo)
	for _, file := range gen.Files {
		if !file.Generate {
			continue
		}
		resources, err := discoverResources(file)
		if err != nil {
			return err
		}
		if len(resources) == 0 {
			continue
		}
		resourcesByFile[file] = resources
		for _, resource := range resources {
			resourcesByName[string(resource.message.Desc.FullName())] = resource
		}
	}
	if err := validateStandardMethods(gen.Files, resourcesByName); err != nil {
		return err
	}
	for file, resources := range resourcesByFile {
		switch target {
		case "go":
			generateGoFile(gen, file, resources)
		case "ts":
			generateTypeScriptFile(gen, file, resources)
		default:
			return fmt.Errorf("crud: unknown target %q (want go or ts)", target)
		}
	}
	return nil
}
