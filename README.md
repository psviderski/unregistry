<div align="center">
  <img src=".github/images/logo-light.svg#gh-light-mode-only" alt="Unregistry logo"/>
  <img src=".github/images/logo-dark.svg#gh-dark-mode-only" alt="Unregistry logo"/>
  <p><strong>▸ Push docker images directly to remote servers without an external registry ◂</strong></p>

  <p>
    <a href="https://discord.gg/eR35KQJhPu"><img src="https://img.shields.io/badge/discord-5865F2.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Join Discord"></a>
    <a href="https://x.com/psviderski"><img src="https://img.shields.io/badge/follow-black?style=for-the-badge&logo=X&logoColor=while" alt="Follow on X"></a>
    <a href="https://github.com/sponsors/psviderski"><img src="https://img.shields.io/badge/Donate-EA4AAA.svg?style=for-the-badge&logo=githubsponsors&logoColor=white" alt="Donate"></a>
  </p>
</div>

Unregistry is a lightweight container image registry that stores and serves images directly from your Docker daemon's
storage.

The included `docker pussh` command (extra 's' for SSH) lets you push images straight to remote Docker servers over SSH.
It transfers only the missing layers, making it fast and efficient.

https://github.com/user-attachments/assets/9d704b87-8e0d-4c8a-9544-17d4c63bd050

## The problem

You've built a Docker image locally. Now you need it on your server. Your options suck:

- **Docker Hub / GitHub Container Registry** - Your code is now public, or you're paying for private repos
- **Self-hosted registry** - Another service to maintain, secure, and pay for storage
- **Save/Load** - `docker save | ssh <remote server> docker load` transfers the entire image, even if 90% already exists on the server
- **Rebuild remotely** - Wastes time and server resources. Plus now you're debugging why the build fails in production

You just want to move an image from A to B. Why is this so hard?

## The solution

```shell
docker pussh myapp:latest user@server
```

That's it. Your image is on the remote server. No registry setup, no subscription, no intermediate storage, no exposed
ports. Just a **direct transfer** of the **missing layers** over SSH.

Here's what happens under the hood:

1. Establishes SSH tunnel to the remote server
2. Starts a temporary unregistry container on the server
3. Forwards a random localhost port to the unregistry port over the tunnel
4. `docker push` to unregistry through the forwarded port, transferring only the layers that don't already exist
   remotely. The transferred image is instantly available on the remote Docker daemon
5. Stops the unregistry container and closes the SSH tunnel

It's like `rsync` for Docker images — simple and efficient.

> [!NOTE]
> Unregistry was created for [Uncloud](https://github.com/psviderski/uncloud), a lightweight tool for deploying
> containers across multiple Docker hosts. We needed something simpler than a full registry but more efficient than
> save/load.

## Requirements

### On local machine

- Docker CLI with plugin support (Docker 19.03+)
- OpenSSH client

### On remote server

- Docker is installed and running
- SSH user has permissions to run `docker` commands (user is `root` or non-root user is in `docker` group — see
  [Manage Docker as a non-root user](https://docs.docker.com/engine/install/linux-postinstall/#manage-docker-as-a-non-root-user)
  for details)
- If `sudo` is required, ensure the user can run `sudo docker` without a password prompt
- Your server has internet access to [ghcr.io](https://ghcr.io) to pull the unregistry image
  `ghcr.io/psviderski/unregistry:latest` on first `docker pussh` use.
    - If your server requires a proxy to access the internet, configure Docker to use it by following the
      [Daemon proxy configuration](https://docs.docker.com/engine/daemon/proxy/) guide.
    - For air-gapped environments or where the access to [ghcr.io](https://ghcr.io) is restricted, you can preload the
      image manually:
      ```shell
      # On a machine with internet access
      docker pull ghcr.io/psviderski/unregistry:latest
      docker save ghcr.io/psviderski/unregistry:latest | ssh user@server docker load
      ```
- Unregistry container requires access to the containerd socket at `/run/containerd/containerd.sock`, so the container
  runs as `root` to have the necessary permissions

## Installation

### macOS/Linux via Homebrew

```shell
brew install psviderski/tap/docker-pussh
```

After installation, to use `docker-pussh` as a Docker CLI plugin (`docker pussh` command) you need to create a symlink:

```shell
mkdir -p ~/.docker/cli-plugins
ln -sf $(brew --prefix)/bin/docker-pussh ~/.docker/cli-plugins/docker-pussh
```

### macOS/Linux via direct download

Download the current version:

```shell
mkdir -p ~/.docker/cli-plugins

# Download the script to the docker plugins directory
curl -sSL https://raw.githubusercontent.com/psviderski/unregistry/v0.2.1/docker-pussh \
  -o ~/.docker/cli-plugins/docker-pussh

# Make it executable
chmod +x ~/.docker/cli-plugins/docker-pussh
```

If you want to download and use the latest version from the main branch:

```shell
curl -sSL https://raw.githubusercontent.com/psviderski/unregistry/main/docker-pussh \
  -o ~/.docker/cli-plugins/docker-pussh
chmod +x ~/.docker/cli-plugins/docker-pussh
```

### Debian

Via unofficial repository packages created and maintained at [unregistry-debian](https://github.com/dariogriffo/unregistry-debian/) by @dariogriffo

You can install unregistry the debian way by running:

```sh
curl -sS https://debian.griffo.io/EA0F721D231FDD3A0A17B9AC7808B4DD62C41256.asc | sudo gpg --dearmor --yes -o /etc/apt/trusted.gpg.d/debian.griffo.io.gpg
echo "deb https://debian.griffo.io/apt $(lsb_release -sc 2>/dev/null) main" | sudo tee /etc/apt/sources.list.d/debian.griffo.io.list
apt install -y unregistry
apt install docker-pussh
```

or in the releases page of the repository [here](https://github.com/dariogriffo/unregistry-debian/releases)

### Windows

Windows is not currently supported, but you can try using [WSL 2](https://docs.docker.com/desktop/features/wsl/)
with the above Linux instructions.

### Verify installation

```shell
docker pussh --help
```

## Completions

### Bash

Download the completion script
```shell
curl -sSL https://raw.githubusercontent.com/psviderski/unregistry/v0.2.0/completions/docker-pussh-completion \
  -o /etc/bash_completion.d/docker-pussh-completion

# To be able to use it in current shell
source /etc/bash_completion.d/docker-pussh-completion
```

### Zsh
Add the following to your .zshrc file:
```shell
autoload -U +X compinit && compinit
autoload -U +X bashcompinit && bashcompinit
source /etc/bash_completion.d/docker-pussh-completion
```
Download the completion script
```shell
curl -sSL https://raw.githubusercontent.com/psviderski/unregistry/v0.2.0/completions/docker-pussh-completion \
  -o /etc/bash_completion.d/docker-pussh-completion

# To be able to use it in current shell
source ~/.zshrc
```

### Fish
Download the completion script
```shell
curl -sSL https://raw.githubusercontent.com/psviderski/unregistry/v0.2.0/completions/docker.fish \
  -o /etc/fish/completions/

# To be able to use it in current shell
source ~/.config/fish/config.fish
```

## ⚠️ Containerd image store configuration

Unregistry stores images directly in containerd's image store, which is the underlying container runtime used by Docker.
However, by default, Docker maintains its own separate storage layer and doesn't directly use images from containerd.

When you enable [containerd image store](https://docs.docker.com/engine/storage/containerd/) in Docker, it allows Docker
to directly use the same images that unregistry stores in containerd, eliminating duplication.

### With containerd image store enabled (recommended)

- Images pushed through unregistry are immediately available to Docker
- No additional storage space is used (images are stored once in containerd)
- Faster `pussh` operations without the additional pull step from unregistry to the classic Docker image store

### Without containerd image store (default Docker behaviour)

- After pushing, `pussh` runs an additional `docker pull` on the remote host to pull the image from unregistry to make
  it available to Docker
- Images are stored twice: once in containerd (by unregistry) and once in the classic Docker image store
- These unmanaged images in containerd can fill up disk space over time. To manage them manually, use:
  ```shell
  sudo ctr -n moby images ls
  sudo ctr -n moby images rm <image>
  ```

### How to enable containerd image store

Please refer to the official Docker documentation:
[Enable containerd image store on Docker Engine](https://docs.docker.com/engine/storage/containerd/#enable-containerd-image-store-on-docker-engine).

> [!WARNING]
> Switching to containerd image store causes you to temporarily lose images and containers created using the classic
> storage driver. Those resources still exist on your filesystem, and you can retrieve them by turning off the
> containerd image store feature.

## Usage

Push an image to a remote server. Please make sure the SSH user has permissions to run `docker` commands (user is
`root` or non-root user is in `docker` group). If `sudo` is required, ensure the user can run `sudo docker` without a
password prompt.

```shell
docker pussh myapp:latest user@server.example.com
```

With SSH key authentication if the private key is not added to your SSH agent:

```shell
docker pussh myapp:latest ubuntu@192.168.1.100 -i ~/.ssh/id_rsa
```

Using a custom SSH port:

```shell
docker pussh myapp:latest user@server:2222
```

Push a specific platform for a multi-platform image. The local Docker has to use
[containerd image store](https://docs.docker.com/desktop/features/containerd/) to support multi-platform images.

```shell
docker pussh myapp:latest user@server --platform linux/amd64
```

## Use cases

### Deploy to production servers

Build locally and push directly to your production servers. No middleman.

```shell
docker build --platform linux/amd64 -t myapp:1.2.3 .
docker pussh myapp:1.2.3 deploy@prod-server
ssh deploy@prod-server docker run -d myapp:1.2.3
```

### CI/CD pipelines

Skip the registry complexity in your pipelines. Build and push directly to deployment targets.

```yaml
- name: Build and deploy
  run: |
    docker build -t myapp:${{ github.sha }} .
    docker pussh myapp:${{ github.sha }} deploy@staging-server
```

### Homelab and air-gapped environments

Distribute images in isolated networks that can't access public registries over the internet.

## Advanced usage

### Running unregistry standalone

Sometimes you want a local registry without the overhead. Unregistry works great for this:

```shell
# Run unregistry locally and expose it on port 5000
docker run -d -p 5000:5000 --name unregistry \
  -v /run/containerd/containerd.sock:/run/containerd/containerd.sock \
  ghcr.io/psviderski/unregistry

# Use it like any registry
docker tag myapp:latest localhost:5000/myapp:latest
docker push localhost:5000/myapp:latest
```

### Custom SSH options

Need custom SSH settings? Use the standard SSH config file:

```shell
# ~/.ssh/config
Host prod-server
    HostName server.example.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/deploy_key

# Now just use
docker pussh myapp:latest prod-server
```

## Contributing

Found a bug or have a feature idea? We'd love your help!

- 🐛 Found a bug? [Open an issue](https://github.com/psviderski/unregistry/issues)

* 💡 Have questions, ideas, or need help?
    * Start a discussion or join an existing one in
      the [Discussions](https://github.com/psviderski/unregistry/discussions).
    * Join the [Uncloud Discord community](https://discord.gg/eR35KQJhPu) where we discuss features, roadmap,
      implementation details, and help each other out.

## Inspiration & acknowledgements

- [Spegel](https://github.com/spegel-org/spegel) - P2P container image registry that inspired me to implement a registry
  that uses containerd image store as a backend.
- [Docker Distribution](https://github.com/distribution/distribution) - the bulletproof Docker registry implementation
  that unregistry is based on.

##

<div align="center">
  Built with ❤️ by <a href="https://github.com/psviderski">Pasha Sviderski</a> who just wanted to deploy his images
</div>
