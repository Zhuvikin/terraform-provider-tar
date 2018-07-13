package main

import (
	"github.com/Zhuvikin/terraform-provider-tar/tar"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: tar.Provider})
}
