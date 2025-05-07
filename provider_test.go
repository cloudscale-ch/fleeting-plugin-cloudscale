package cloudscale

import (
	"testing"

	cloudscale "github.com/cloudscale-ch/cloudscale-go-sdk/v6"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/fleeting/fleeting/provider"
)

func TestInstanceGroupValidation(t *testing.T) {

	assertErrorContains := func(g InstanceGroup, contains string) {
		_, err := g.Init(t.Context(), hclog.Default(), provider.Settings{})
		assert.ErrorContains(t, err, contains)
	}

	group := InstanceGroup{}
	assertErrorContains(group, "api_token: required field")
	assertErrorContains(group, "group: required field")
	assertErrorContains(group, "group name must match")
	assertErrorContains(group, "flavor: required field")
	assertErrorContains(group, "image: required field")
	assertErrorContains(group, "image: required field")
	assertErrorContains(group, "volume_size_gb must be >= 10")
}

func TestMinimalValidInstanceGroup(t *testing.T) {

	group := InstanceGroup{
		Group:        "fleeting",
		ApiToken:     "test-token",
		Flavor:       "flex-4-2",
		Image:        "ubuntu-24.04",
		VolumeSizeGB: 10,
	}

	info, err := group.Init(t.Context(), hclog.Default(), provider.Settings{})

	assert.NoError(t, err)
	assert.Equal(t, info.Version, "dev")
	assert.Equal(t, info.ID, "cloudscale/fleeting")
}

func TestRandomServerName(t *testing.T) {

	group := InstanceGroup{
		Group:        "fleeting",
		ApiToken:     "test-token",
		Flavor:       "flex-4-2",
		Image:        "ubuntu-24.04",
		VolumeSizeGB: 10,
	}

	_, err := group.Init(
		t.Context(), hclog.Default(), provider.Settings{})

	assert.NoError(t, err)
	assert.Regexp(t, "^fleeting-[a-z0-9]+$", group.serverName())

	for i := 0; i < 100; i++ {
		assert.NotEqual(t, group.serverName(), group.serverName())
		assert.NoError(t, group.ensureExpectedServerName(group.serverName()))
	}
}

func TestPublicKey(t *testing.T) {

	group := InstanceGroup{
		Group:        "fleeting",
		ApiToken:     "test-token",
		Flavor:       "flex-4-2",
		Image:        "ubuntu-24.04",
		VolumeSizeGB: 10,
	}

	_, err := group.Init(
		t.Context(), hclog.Default(), provider.Settings{})

	key, err := group.publicKey()
	assert.NoError(t, err)

	assert.Regexp(t, "^ssh-.+ fleeting-plugin-cloudscale\n$", string(key))
}

func TestTagMap(t *testing.T) {

	group := InstanceGroup{
		Group:        "fleeting",
		ApiToken:     "test-token",
		Flavor:       "flex-4-2",
		Image:        "ubuntu-24.04",
		VolumeSizeGB: 10,
	}

	_, err := group.Init(
		t.Context(), hclog.Default(), provider.Settings{})

	assert.NoError(t, err)

	group.Group = "fleeting"
	tags := group.tagMap()
	assert.Equal(t, tags["fleeting-instance-group"], "fleeting")
	assert.NoError(t, group.ensureExpectedTagMap(cloudscale.TagMap{
		"fleeting-instance-group": "fleeting",
	}))

	group.Group = "alt"
	tags = group.tagMap()
	assert.Equal(t, tags["fleeting-instance-group"], "alt")
	assert.NoError(t, group.ensureExpectedTagMap(cloudscale.TagMap{
		"fleeting-instance-group": "alt",
	}))
}

func TestSafeToDelete(t *testing.T) {

	group := InstanceGroup{
		Group:        "fleeting",
		ApiToken:     "test-token",
		Flavor:       "flex-4-2",
		Image:        "ubuntu-24.04",
		VolumeSizeGB: 10,
	}

	_, err := group.Init(
		t.Context(), hclog.Default(), provider.Settings{})

	assert.NoError(t, err)

	server := cloudscale.Server{
		Name: "fleeting-0123456789",
		TaggedResource: cloudscale.TaggedResource{
			Tags: cloudscale.TagMap{
				"fleeting-instance-group": "fleeting",
			},
		},
	}

	assert.NoError(t, group.ensureSafeToDelete(&server))

	server = cloudscale.Server{
		Name: "prod-db",
	}

	assert.Error(t, group.ensureSafeToDelete(&server))

	server = cloudscale.Server{
		Name: "prod-db",
		TaggedResource: cloudscale.TaggedResource{
			Tags: cloudscale.TagMap{
				"fleeting-instance-group": "fleeting",
			},
		},
	}

	assert.Error(t, group.ensureSafeToDelete(&server))

	server = cloudscale.Server{
		Name: "fleeting-0123456789",
		TaggedResource: cloudscale.TaggedResource{
			Tags: cloudscale.TagMap{
				"fleeting-instance-group": "fleeting-alt",
			},
		},
	}

	assert.Error(t, group.ensureSafeToDelete(&server))
}
