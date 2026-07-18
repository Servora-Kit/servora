package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/Servora-Kit/servora/cmd/protoc-gen-typescript-http/internal/plugin"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", filepath.Base(os.Args[0]), err)
		os.Exit(1)
	}
}

func run() error {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err = proto.Unmarshal(in, req); err != nil {
		return err
	}
	resp, err := plugin.Generate(req)
	if err != nil {
		// Even on fatal error, return a response with Error field set
		// so protoc can display the error in its standard format.
		errResp := &pluginpb.CodeGeneratorResponse{
			Error: proto.String(fmt.Sprintf("%s: %v", filepath.Base(os.Args[0]), err)),
		}
		if out, marshalErr := proto.Marshal(errResp); marshalErr == nil {
			_, _ = os.Stdout.Write(out)
		}
		return err
	}
	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	return nil
}
