<div align="center">
  <img src=".github/images/logo-light.svg#gh-light-mode-only" alt="Unregistry logo"/>
  <img src=".github/images/logo-dark.svg#gh-dark-mode-only" alt="Unregistry logo"/>
  <p><strong>▸ Push docker images directly to remote servers without an external registry ◂</strong></p>

  <p>
    <a href="https://discord.gg/eR35KQJhPu"><img src="https://img.shields.io/badge/discord-5865F2.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Join Discord"></a>
    <a href="https://x.com/psviderski"><img src="https://img.shields.io/badge/follow-black?style=for-the-badge&logo=X&logoColor=while" alt="Follow on X"></a>
  </p>
</div>

**Unregistry** is a lightweight container image registry that serves images directly from your Docker/containerd 
daemon's storage.

The included `docker pussh` command allows you to push an image straight to a remote Docker daemon via SSH. It starts
a temporary unregistry container on the remote host to transfer only the missing image layers. Your image transfers
directly to where you need to run it, without going through an external registry.

https://github.com/user-attachments/assets/9d704b87-8e0d-4c8a-9544-17d4c63bd050

## Why unregistry?

You've built a Docker image locally. Now you need it on your server. Your options suck:

- **Docker Hub / GitHub Container Registry** - Your code is now public, or you're paying for private repos
- **Self-hosted registry** - Another service to maintain, secure, and pay for storage
- **Save/Load** - `docker save | ssh | docker load` is slow and inefficient, especially for large images
- **Rebuild remotely** - Wastes time and server resources. Who builds images on production servers?

You just want to move an image from A to B. Why is this so hard?

**That's why we built unregistry.** One command replaces the entire workflow:

```bash
docker pussh myapp:latest user@server
```

Your image is now on the remote server. No registry account, no subscription, no intermediate storage, no open ports to
the world. Just a **direct transfer** of the **missing image layers** over SSH.

> [!NOTE]  
> Unregistry was originally created for [Uncloud](https://github.com/psviderski/uncloud), a lightweight tool for 
> deploying and managing containerised applications across a network of Docker hosts. We needed a simple way to upload
> locally built images to remote machines without requiring a registry. And realised this problem deserves its own
> solution.


##
<div align="center">
  Built with ❤️ by <a href="https://github.com/psviderski">Pasha Sviderski</a> who just wanted to deploy his images
</div>
