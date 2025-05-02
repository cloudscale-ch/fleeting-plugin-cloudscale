# Fleeting Plugin Cloudscale

A [fleeting](https://docs.gitlab.com/runner/fleet_scaling/fleeting/) plugin for [cloudscale.ch](https://www.cloudscale.ch), for automatically scaled GitLab runners.

## ✅ Features

- 🚀 Automatically provide GitLab Runner capacity with cloudscale.ch VMs.
- ♻️ Provide capacity when it is needed, reduce costs when it is not.
- 🧩 Well integrated into self-managed GitLab instances.

### Caveats

- ⬇️ Stopped runner instances are automatically removed and recycled.

## 🏎️ Demo and Ansible Playbook

To take this setup for a test-drive, use our Ansible playbook that installs a self-managed GitLab instance and automatically configures our Fleeting plugin:

https://github.com/cloudscale-ch/gitlab-runner

You can use this as a demo, or as a starting point to introduce GitLab into your own infrastructure.

## 💪 Installation

To get started, install GitLab according to GitLab's official documentation:

https://about.gitlab.com/install/

Next, you need at least one GitLab runner that is always on. This runner will be responsible for launching other runners, as needed. It can be run anywhere and can also run its own jobs.

A good option, so that you do not have to use extra resources for this runner, is to simply install it on the same host as your GitLab server. Here, you want a runner that has no other jobs (or its CI jobs will compete with your GitLab server for resources).

Either way, the following applies whether you install this runner on your GitLab server host, or in a dedicated VM or container.

### Install GitLab Runner

There are multiple ways to install GitLab runner:

https://docs.gitlab.com/runner/install/

We recommend using a distro-specific package, as it makes updates easier:

https://docs.gitlab.com/runner/install/linux-repository.html

Please skip the "Register a runner" step, as this will be done with a more specific call further down.

### Acquire GitLab Runner Registration Token

To register your runner, you need a registration token. This can be a global token (to create a shared runner) or a project-specific token.

For a shared runner, open the admin dashboard at `/admin`, and select "Runners" from the left (`/admin/runners`). Clicking on "New instance runner" allows you to define a new runner.

Note that to actually register the runner, you should not just run the command shown by GitLab, but rather the command below.

### Register GitLab Runner

The following command will register the GitLab runner. It won't work yet until the example config has been applied, and it won't show as online until then.

Use the token shown to you in the "New instance runner" wizard:

```bash
# The endpoint of your GitLab instance
export GITLAB_URL="https://<your-gitlab-instance>"

# The runner token shown in the wizard
export RUNNER_TOKEN="glpat-..."

sudo gitlab-runner register \
   --non-interactive \
   --url "$GITLAB_URL" \
   --registration-token "$RUNNER_TOKEN" \
   --executor "docker-autoscaler" \
   --docker-image alpine
```

### Configure GitLab Runner

Create a backup of current config, then download the config template and fill it out with the editor of your choice:

```bash
# Create backup
cp /etc/gitlab-runner/config.toml /etc/gitlab-runner/config.toml.bak

# Download template
sudo curl -sL https://raw.githubusercontent.com/cloudscale-ch/fleeting-plugin-cloudscale/refs/heads/main/config/config.toml.template -o /etc/gitlab-runner/config.toml

# Edit config
vim /etc/gitlab-runner/config.toml
```

You should at least change the following values:

- `$GITLAB_URL`
- `$RUNNER_TOKEN`
- `$CLOUDSCALE_API_TOKEN`

The comments in the file should help you decide on good defaults for your specific use-case.

### Install Fleeting Plugin

Having configured your runner thusly, install `fleeting-plugin-cloudscale` as defined in the config file:

```bash
sudo gitlab-runner fleeting install
```

> [!IMPORTANT]  
> Note the sudo - fleeting will fail to install the plugin without it.

Your runner should pick up config changes automatically. It may have to create its first autoscaled instance, before it is shown as online on your GitLab instance, but after that it should work without intervention.

The log may be helpful, if you run into trouble:

```
sudo journalctl -u gitlab-runner --since today
```

### Upgrades

To upgrade to a newer release of `fleeting-plugin-cloudscale`, run the following:

```bash
gitlab-runner fleeting install --upgrade
systemctl restart gitlab-runner
```

> [!WARNING]  
> This will recreate all runners, so you might want to do this outside your peak CI hours.

## ⚙️ Plugin Configuration

The following parameters are supported:

| Parameter        | Type   | Description                                                                                                     |
| ---------------- | ------ | --------------------------------------------------------------------------------------------------------------- |
| `group`          | string | The name of the scheduling group.                                                                               |
| `api_token`      | string | cloudscale API token                                                                                            |
| `zone`           | string | Zone where virtual machine is started, [available zones](https://www.cloudscale.ch/en/api/v1#regions-and-zones) |
| `flavor`         | string | Flavor of virtual machine, [available flavors](https://www.cloudscale.ch/en/pricing)                            |
| `image`          | string | Image (slug) of virtual machine, [more information](https://www.cloudscale.ch/en/api/v1#images)                 |
| `volume_size_gb` | number | The size of the root volume in GiB. Must be at least `10`.                                                      |
| `user_data`      | string | Depending on the image choice is either in the format of cloud-init (YAML) or Ignition (JSON).                  |
| `network`        | string | The primary network to attach to the machines. Defaults to `public`. You need to ensure the runner controller can reach the machines via this one.                                                      |
| `extra_networks` | string | Additional networks to be added to the machines.                                                                |

### Default connector config

| Parameter                | Default                           |
| ------------------------ | --------------------------------- |
| `os`                     | `linux`                           |
| `protocol`               | `ssh` **(only ssh is supported)** |
| `username`               | `root` for custom images          |
| `use_static_credentials` | `false`                           |

Note: The `username` can be left empty when using an official image (from `/v1/images`). When using a custom image, the user defaults to `root` and should likely be configured explicitly.

## 💡 Development

To test the plugin locally, you can use the provided `compose.yml`.

### Setup

1. **Create a custom configuration file**
   A template configuration is available in the `/config` directory.
   Copy and modify it to match your environment:

   ```sh
   cp ./config/config.toml.template ./config/config.toml
   ```

   Note: The configuration template configures a "docker-autoscaler".

2. **Update the configuration**
   Edit ./config/config.toml and set the following variables:

   - `$GITLAB_URL` - Your GitLab instance URL.
   - `$RUNNER_TOKEN` - Your GitLab runner registration token.
   - `$CLOUDSCALE_API_TOKEN` - Your Cloudscale API token.

### Running the Plugin Locally

Once the configuration is set up, you can start a local runner with:

```sh
docker compose up --watch
```

This will launch the runner with automatic rebuilds on changes.

Note that you need to change the `plugin` in `config/config.toml` to the following:

```toml
plugin = "fleeting-plugin-cloudscale"
```

### Go Tools

This repository uses `go tool` to run and version development tools. They are tracked in the separate `tool.mod` modfile.

To list all tools:

```bash
go tool -modfile tool.mod
```

To add a tool to tool.mod:

```bash
go get -tool -modfile tool.md <url>
```

To install the tool:

```bash
go mod tidy -modfile tool.md
```

### Release Process

To create a new release, simply tag the commit that should become the release:

```bash
git tag <major>.<minor>.<patch>
git push
git push --tags 
```

Once all CI jobs have completed, be sure to check the generated release notes, and maybe editorialize them somewhat.
