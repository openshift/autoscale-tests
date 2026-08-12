package main

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/testgrid/config/yamlcfg"
	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s input.yaml output.pb\n", os.Args[0])
		os.Exit(1)
	}

cfg, err := yamlcfg.ReadConfig([]string{os.Args[1]}, "testgrid-demo/default.yaml", false,)
	if err != nil {
		panic(err)
	}

	data, err := proto.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(os.Args[2], data, 0644); err != nil {
		panic(err)
	}
}
