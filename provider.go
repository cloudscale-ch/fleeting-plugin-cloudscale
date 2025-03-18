package cloudscale

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path"

	cloudscale "github.com/cloudscale-ch/cloudscale-go-sdk/v5"
	"github.com/google/uuid"
	hclog "github.com/hashicorp/go-hclog"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"golang.org/x/crypto/ssh"
)

var _ provider.InstanceGroup = (*InstanceGroup)(nil)

type InstanceGroup struct {
	Group string `json:"group"`

	ApiToken string `json:"api_token"`

	Zone     string `json:"zone"`
	Flavor   string `json:"flavor"`
	Image    string `json:"image"`
	UserData string `json:"user_data"`

	VolumeSizeGB int `json:"volume_size_gb,omitempty"`

	log      hclog.Logger
	settings provider.Settings

	client *cloudscale.Client
}

func (g *InstanceGroup) publicKey() ([]byte, error) {

	priv, err := ssh.ParseRawPrivateKey(g.settings.Key)
	if err != nil {
		return nil, fmt.Errorf("reading private key: %w", err)
	}

	// see: https://pkg.go.dev/crypto#PrivateKey
	type PrivPub interface {
		crypto.PrivateKey
		Public() crypto.PublicKey
	}

	privateKey, ok := priv.(PrivPub)
	if !ok {
		return nil, fmt.Errorf("key doesn't export Public()")
	}

	sshPubKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		return nil, fmt.Errorf("generating ssh public key: %w", err)
	}

	// Generate the authorizd key, together with a note that it came
	// from this plugin.
	marshaled := ssh.MarshalAuthorizedKey(sshPubKey)
	marshaled = bytes.TrimSuffix(marshaled, []byte("\n"))

	return append(marshaled, []byte(" fleeting-plugin-cloudscale\n")...), nil
}

func (g *InstanceGroup) tagMap() cloudscale.TagMap {
	return cloudscale.TagMap{
		"fleeting-instance-group": g.Group,
	}
}

func (g *InstanceGroup) serverName() string {
	return fmt.Sprintf("%s-%s", g.Group, uuid.NewString())
}

// Init initializes the ProviderInfo struct
func (g *InstanceGroup) Init(ctx context.Context, logger hclog.Logger, settings provider.Settings) (info provider.ProviderInfo, err error) {
	g.settings = settings
	g.log = logger.Named("fleeting-plugin-cloudscale").With(
		"group", g.Group,
		"zone", g.Zone)

	if err := g.validate(); err != nil {
		return provider.ProviderInfo{}, err
	}

	g.client = cloudscale.NewClient(http.DefaultClient)
	g.client.AuthToken = g.ApiToken

	if g.settings.Key == nil {
		g.log.Info("generating SSH private key")

		_, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return provider.ProviderInfo{},
				fmt.Errorf("failed to generate SSH private key: %w", err)
		}

		privPem, err := ssh.MarshalPrivateKey(priv, "")
		if err != nil {
			return provider.ProviderInfo{},
				fmt.Errorf("failed to marshal SSH private key: %w", err)
		}

		g.settings.Key = pem.EncodeToMemory(privPem)
	}

	if _, err := g.client.Servers.List(ctx); err != nil {
		return provider.ProviderInfo{},
			fmt.Errorf("failed to initialize client: %w", err)
	}

	return provider.ProviderInfo{
		ID:        path.Join("cloudscale", g.Zone, g.Flavor),
		MaxSize:   math.MaxInt,
		Version:   Version.String(),
		BuildInfo: Version.BuildInfo(),
	}, nil
}

// Update updates instance data from the instance group, passing a function
// to perform instance reconciliation.
func (g *InstanceGroup) Update(ctx context.Context, update func(instance string, state provider.State)) error {
	servers, err := g.client.Servers.List(ctx, cloudscale.WithTagFilter(g.tagMap()))
	if err != nil {
		return err
	}

	for _, server := range servers {
		id := server.UUID
		var state provider.State

		switch server.Status {
		case string(cloudscale.ServerStopped):
			state = provider.StateDeleted
		case string(cloudscale.ServerRunning):
			state = provider.StateRunning
		case "changing":
			state = provider.StateCreating
		default:
			g.log.Error("unexpected instance status", "id", id, "status", server.Status)
		}

		update(id, state)
	}

	return nil
}

// Increase requests more instances to be created. It returns how many
// instances were successfully requested.
func (g *InstanceGroup) Increase(ctx context.Context, delta int) (succeeded int, err error) {
	servers := make([]*cloudscale.Server, 0, delta)
	errs := make([]error, 0)

	publicKey, err := g.publicKey()
	if err != nil {
		return 0, err
	}

	tagMap := g.tagMap()

	for i := 0; i < delta; i++ {
		serverName := g.serverName()

		g.log.Info("creating server", "name", serverName, "flavor", g.Flavor)
		server, err := g.client.Servers.Create(ctx, &cloudscale.ServerRequest{
			Name:         serverName,
			Zone:         g.Zone,
			Flavor:       g.Flavor,
			Image:        g.Image,
			SSHKeys:      []string{string(publicKey)},
			VolumeSizeGB: g.VolumeSizeGB,
			UserData:     g.UserData,
			TaggedResourceRequest: cloudscale.TaggedResourceRequest{
				Tags: &tagMap,
			},
		})

		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to create %s: %w", serverName, err))
			continue
		}

		server, err = g.client.Servers.WaitFor(
			ctx, server.UUID, cloudscale.ServerIsRunning)

		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to wait for %s: %w", serverName, err))
			continue
		}

		g.log.Info("created server", "name", serverName, "uuid", server.UUID)
		servers = append(servers, server)
	}

	return len(servers), errors.Join(errs...)

}

// Decrease removes the specified instances from the instance group. It
// returns instance IDs of successful requests for removal.
func (g *InstanceGroup) Decrease(ctx context.Context, ids []string) (succeeded []string, err error) {
	errs := make([]error, 0)

	for _, id := range ids {
		g.log.Info("deleting server", "uuid", id)

		err := g.client.Servers.Delete(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to delete server %s: %w", id, err))
			continue
		}
		succeeded = append(succeeded, id)
	}

	return succeeded, errors.Join(errs...)
}

// ConnectInfo returns additional information about an instance,
// useful for creating a connection.
func (g *InstanceGroup) ConnectInfo(ctx context.Context, id string) (provider.ConnectInfo, error) {
	info := provider.ConnectInfo{ConnectorConfig: g.settings.ConnectorConfig}

	server, err := g.client.Servers.Get(ctx, id)
	if err != nil {
		return info, fmt.Errorf("unable to fetch server %s: %w", id, err)
	}

	info.ID = server.UUID
	info.OS = server.Image.OperatingSystem

	// cloudscale.ch only supports amd64
	info.Arch = "amd64"

	if server.Image.DefaultUsername != "" {
		info.Username = server.Image.DefaultUsername
	} else {
		info.Username = g.settings.Username
	}

	info.Key = g.settings.Key
	info.ExternalAddr = server.Interfaces[0].Addresses[0].Address // Only public address is supported at the moment.

	return info, nil
}

// Shutdown performs any cleanup tasks required when the plugin is to shutdown.
func (g *InstanceGroup) Shutdown(ctx context.Context) error {
	// No cleanup needed,
	// as we do not need to upload any keys before creating a server.
	return nil
}
