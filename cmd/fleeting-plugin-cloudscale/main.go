package main

import (
	cloudscale "github.com/puzzle/fleeting-plugin-cloudscale"
	"gitlab.com/gitlab-org/fleeting/fleeting/plugin"
)

func main() {
	plugin.Main(&cloudscale.InstanceGroup{}, cloudscale.Version)
}
