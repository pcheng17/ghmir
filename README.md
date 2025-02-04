# ghmir

## Dependencies

This tool depends on [ghorg](https://github.com/gabrie30/ghorg) as this is just a thin wrapper
around that.

## Installation

### Using go install

```bash
go install github.com/pcheng17/ghmir@latest
```

### Binary Downloads

Download the pre-compiled binary from the releases page.

#### Linux/macOS
```bash
# Replace VERSION with the version you want to install
curl -L https://github.com/yourusername/ghmir/releases/download/vVERSION/ghmir_Linux_x86_64.tar.gz | tar xz
sudo mv ghmir /usr/local/bin/
```

#### Windows
Download the ZIP file from the releases page and extract it to a location in your PATH.

## Secrets

When using `ghmir`, one can specify a YAML file containing the necessary secrets via `--secrets`, or
if one is not specified, then `ghmir` will by default look for `$HOME/.config/ghmir/secrets.yaml`.
The YAML file should look like the following:

```
discord_webhook: "discord-webhook"

entities:
  entityA:
    github_token: "github_secret_token"
    type: "user|org"
    gitlab:
      token_name: "gitlab_token_name"
      token: "gitlab_secret_token
      group_name: "gitlab_group_name"

  entityB:
    github_token: "github_secret_token"
    type: "user|org"
    gitlab:
      token_name: "gitlab_token_name"
      token: "gitlab_secret_token"
      group_name: "gitlab_group_name"
```
