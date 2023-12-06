package main

import (
	"context"
	"log"

	"terraform-provider-aap/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	// Example version string that can be overwritten by a release process
	version string = "dev"
)

func main() {
	opts := providerserver.ServeOpts{
		// TODO: Update this string with the published name of your provider.
		Address: "registry.terraform.io/ansible/aap",
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}
