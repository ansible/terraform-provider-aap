package main

import (
	"context"
	"flag"
	"log"

	"github.com/ansible/terraform-provider-aap/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	// Example version string that can be overwritten by a release process
	version string = "dev"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/ansible/aap",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}
