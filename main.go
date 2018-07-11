package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/Zhuvikin/terraform-provider-tar/tar"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: tar.Provider})
}
