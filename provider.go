package cloudscale

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path"

	cloudscaleclient "github.com/cloudscale-ch/cloudscale-go-sdk/v5"
	"github.com/google/uuid"
	hclog "github.com/hashicorp/go-hclog"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"golang.org/x/crypto/ssh"
)

var _ provider.InstanceGroup = (*InstanceGroup)(nil)

type InstanceGroup struct {
	Name string `json:"name"`

	ApiToken string `json:"api_token"`

	Zone     string `json:"zone"`
	Flavor   string `json:"flavor"`
	Image    string `json:"image"`
	UserData string `json:"user_data"`

	VolumeSizeGB int `json:"volume_size_gb,omitempty"`

	log      hclog.Logger
	settings provider.Settings

	size int

	client *cloudscaleclient.Client
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

	return ssh.MarshalAuthorizedKey(sshPubKey), nil
}

func (g *InstanceGroup) tagMap() cloudscaleclient.TagMap {
	return cloudscaleclient.TagMap{
		"fleeting-instance-group": g.Name,
	}
}

func (g *InstanceGroup) serverName() string {
	return fmt.Sprintf("%s-%s", g.Name, uuid.NewString())
}

// Init implements provider.InstanceGroup.
func (g *InstanceGroup) Init(ctx context.Context, logger hclog.Logger, settings provider.Settings) (info provider.ProviderInfo, err error) {
	g.settings = settings
	g.log = logger.Named("fleeting-plugin-cloudscale")

	if err := g.validate(); err != nil {
		return provider.ProviderInfo{}, err
	}

	g.client = cloudscaleclient.NewClient(http.DefaultClient)
	g.client.AuthToken = g.ApiToken

	if g.settings.Key == nil {
		g.log.Info("generating SSH key pair")

		_, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return provider.ProviderInfo{}, fmt.Errorf("generating ssh key pair: %w", err)
		}

		privPem, err := ssh.MarshalPrivateKey(priv, "")
		if err != nil {
			return provider.ProviderInfo{}, err
		}

		g.settings.Key = pem.EncodeToMemory(privPem)
	}

	if _, err := g.client.Servers.List(ctx); err != nil {
		return provider.ProviderInfo{}, fmt.Errorf("creating client: %w", err)
	}

	return provider.ProviderInfo{
		ID:        path.Join("cloudscale", g.Zone, g.Flavor),
		MaxSize:   math.MaxInt,
		Version:   Version.String(),
		BuildInfo: Version.BuildInfo(),
	}, nil
}

// Update implements provider.InstanceGroup.
func (g *InstanceGroup) Update(ctx context.Context, update func(instance string, state provider.State)) error {
	servers, err := g.client.Servers.List(ctx, cloudscaleclient.WithTagFilter(g.tagMap()))
	if err != nil {
		return err
	}

	g.size = len(servers)

	for _, server := range servers {
		id := server.UUID
		var state provider.State

		switch server.Status {
		case string(cloudscaleclient.ServerStopped):
			state = provider.StateDeleted
		case string(cloudscaleclient.ServerRunning):
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

// Increase implements provider.InstanceGroup.
func (g *InstanceGroup) Increase(ctx context.Context, delta int) (succeeded int, err error) {
	servers := make([]*cloudscaleclient.Server, 0, delta)
	errs := make([]error, 0)

	publicKey, err := g.publicKey()
	if err != nil {
		return 0, err
	}

	tagMap := g.tagMap()

	for i := 0; i < delta; i++ {
		server, err := g.client.Servers.Create(ctx, &cloudscaleclient.ServerRequest{
			Name:         g.serverName(),
			Zone:         g.Zone,
			Flavor:       g.Flavor,
			Image:        g.Image,
			SSHKeys:      []string{string(publicKey)},
			VolumeSizeGB: g.VolumeSizeGB,
			UserData:     g.UserData,
			TaggedResourceRequest: cloudscaleclient.TaggedResourceRequest{
				Tags: &tagMap,
			},
		})

		if err != nil {
			errs = append(errs, err)
			continue
		}
		servers = append(servers, server)
	}

	g.size += len(servers)
	return len(servers), errors.Join(errs...)

}

// Decrease implements provider.InstanceGroup.
func (g *InstanceGroup) Decrease(ctx context.Context, ids []string) (succeeded []string, err error) {
	errs := make([]error, 0)

	for _, id := range ids {
		err := g.client.Servers.Delete(ctx, id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		succeeded = append(succeeded, id)
	}

	return succeeded, errors.Join(errs...)
}

// ConnectInfo implements provider.InstanceGroup.
func (g *InstanceGroup) ConnectInfo(ctx context.Context, instance string) (provider.ConnectInfo, error) {
	info := provider.ConnectInfo{ConnectorConfig: g.settings.ConnectorConfig}

	server, err := g.client.Servers.Get(ctx, instance)
	if err != nil {
		return info, fmt.Errorf("unable to fetch server: %w", err)
	}

	info.ID = server.UUID
	info.OS = server.Image.OperatingSystem

	// cloudscale.ch only supports amd64
	info.Arch = "amd64"

	info.Username = g.settings.Username

	info.Key = g.settings.Key
	info.ExternalAddr = server.Interfaces[0].Addresses[0].Address // Only public address is supported at the moment.

	return info, nil
}

// Shutdown implements provider.InstanceGroup.
func (g *InstanceGroup) Shutdown(ctx context.Context) error {
	// No cleanup needed,
	// as we do not need to upload any keys before creating a server.
	return nil
}
