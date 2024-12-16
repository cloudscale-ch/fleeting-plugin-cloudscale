# Fleeting Plugin Cloudscale

A [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) plugin for [cloudscale.ch](https://www.cloudscale.ch/en).

## Plugin Configuration

The following parameters are supported:

| Parameter        | Type   | Description                                                                                                     |
| ---------------- | ------ | --------------------------------------------------------------------------------------------------------------- |
| `name`           | string | Name of the instance group                                                                                      |
| `token`          | string | cloudscale API token                                                                                            |
| `zone`           | string | Zone where virtual machine is started, [available zones](https://www.cloudscale.ch/en/api/v1#regions-and-zones) |
| `flavor`         | string | Flavor of virtual machine, [available flavors](https://www.cloudscale.ch/en/pricing)                            |
| `image`          | string | Image (slug) of virtual machine, [more information](https://www.cloudscale.ch/en/api/v1#images)                 |
| `volume_size_gb` | number | The size of the root volume in GiB. Must be at least `10`.                                                      |
| `user_data`      | string | Depending on the image choice is either in the format of cloud-init (YAML) or Ignition (JSON).                  |

### Default connector config

| Parameter                | Default                           |
| ------------------------ | --------------------------------- |
| `os`                     | `linux`                           |
| `protocol`               | `ssh` **(only ssh is supported)** |
| `username`               | `root`                            |
| `use_static_credentials` | `false`                           |

## Development

To test the plugin locally, you can use the provided `compose.yml`.

### Setup

1. **Create a custom configuration file**
   A template configuration is available in the `/hack` directory.
   Copy and modify it to match your environment:

   ```sh
   cp ./hack/config.toml.template ./hack/config.toml
   ```

   Note: The configuration template configures a "docker-autoscaler".

2. **Update the configuration**
   Edit ./hack/config.toml and set the following variables:

   - `$GITLAB_URL` - Your GitLab instance URL.
   - `$RUNNER_TOKEN` - Your GitLab runner registration token.
   - `$CLOUDSCALE_API_TOKEN` - Your Cloudscale API token.

### Running the Plugin Locally

Once the configuration is set up, you can start a local runner with:

```sh
docker compose up --watch
```

This will launch the runner with automatic rebuilds on changes.
