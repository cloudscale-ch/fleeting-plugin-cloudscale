package cloudscale

import (
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
)

func (g *InstanceGroup) validate() error {
	// adopted from https://gitlab.com/hetznercloud/fleeting-plugin-hetzner/-/blob/1687c2a89eaa47084b477d0bbf20d1a371da6a37/config.go#L11
	errs := []error{}

	// Defaults
	if g.settings.Protocol == "" {
		g.settings.Protocol = provider.ProtocolSSH
	}

	if g.settings.Username == "" {
		g.settings.Username = "root"
	}

	// Checks
	if g.ApiToken == "" {
		errs = append(errs, fmt.Errorf("missing required plugin config: api_token"))
	}

	if g.Group == "" {
		errs = append(errs, fmt.Errorf("missing required plugin config: group"))
	}

	if g.Zone != "" && (g.Zone != "rma1" && g.Zone != "lpg1") {
		errs = append(errs, fmt.Errorf("unsupported zone slug \"%s\" should be either \"rma1\" or \"lpg1\"", g.Zone))
	}

	if g.Flavor == "" {
		errs = append(errs, fmt.Errorf("missing required plugin config: flavor"))
	}

	if g.Image == "" {
		errs = append(errs, fmt.Errorf("missing required plugin config: image"))
	}

	if g.VolumeSizeGB < 10 {
		errs = append(errs, fmt.Errorf("invalid plugin configuration: volume_size_gb must be >= 10"))
	}

	if g.settings.Protocol == provider.ProtocolWinRM {
		errs = append(errs, fmt.Errorf("invalid plugin config:  SSH is the only supported protocol"))
	}

	if g.settings.UseStaticCredentials && g.settings.Key == nil {
		errs = append(errs, fmt.Errorf("invalid plugin config: use_static_credentials is true but no key is configured"))
	}

	return errors.Join(errs...)
}
