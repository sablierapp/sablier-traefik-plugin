<!-- omit in toc -->
# Traefik Sablier Plugin

[![Go Report Card](https://goreportcard.com/badge/github.com/sablierapp/sablier-traefik-plugin)](https://goreportcard.com/report/github.com/sablierapp/sablier-traefik-plugin)
[![Discord](https://img.shields.io/discord/1298488955947454464?logo=discord&logoColor=5865F2&cacheSeconds=1&link=http%3A%2F%2F)](https://discord.gg/Gpz5ha8WWx)

<img src="./docs/assets/img/traefik.png" alt="traefik" width="200" align="right" />

Automatically start containers on demand and shut them down during periods of inactivity using [Traefik](https://github.com/traefik/traefik).

This plugin is available in the [Traefik Plugin Catalog](https://plugins.traefik.io/plugins/69104ac3b7d4dd76110a1a09/sablier).

- [Installation](#installation)
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Dynamic Configuration](#dynamic-configuration)
  - [Docker](#docker)
  - [Docker Swarm](#docker-swarm)
  - [Kubernetes](#kubernetes)
- [Other Reverse Proxy Plugins](#other-reverse-proxy-plugins)
  - [Apache APISIX](#apache-apisix)
  - [Caddy](#caddy)
  - [Envoy](#envoy)
  - [Istio](#istio)
  - [Nginx](#nginx)
- [Community](#community)
- [Support](#support)

## Installation

Add the following snippet to your Traefik static configuration:

<!-- x-release-please-version-start -->
```yaml
experimental:
  plugins:
    sablier:
      moduleName: "github.com/sablierapp/sablier-traefik-plugin"
      version: "v1.0.0"
```

Or using CLI arguments:

```yaml
--experimental.plugins.sablier.moduleName=github.com/sablierapp/sablier-traefik-plugin
--experimental.plugins.sablier.version=v1.0.0
```
<!-- x-release-please-version-end -->

## Prerequisites

> [!IMPORTANT]
> You must have a reachable instance of [Sablier](https://github.com/sablierapp/sablier) from [Traefik](https://github.com/traefik/traefik).

## Configuration

The plugin supports various configuration options to customize its behavior. All options are configured at the middleware level.


**Complete Configuration Example:**

```yaml
http:
  middlewares:
    my-sablier:
      plugin:
        sablier:
          sablierUrl: http://sablier:10000
          group: my-app-group
          sessionDuration: 1m
          ignoreUserAgent: curl
          dynamic:
            displayName: My Services
            showDetails: true
            theme: hacker-terminal
            refreshFrequency: 5s
```

| Option            | Type     | Required | Default                | Description                                                                         |
| ----------------- | -------- | -------- | ---------------------- | ----------------------------------------------------------------------------------- |
| `sablierUrl`      | string   | Yes      | `http://sablier:10000` | URL of the Sablier server API (must be reachable from Traefik)                      |
| `group`           | string   | Yes      | -                      | Group name for managing multiple instances collectively                         >    |
| `sessionDuration` | duration | No       | -                      | Duration to keep instances running after the last request (e.g., `1m`, `30s`, `2h`) |
| `dynamic.displayName`      | string   | No       | Middleware name | Display name shown on the waiting page                                  |
| `dynamic.showDetails`      | boolean  | No       | Server default  | Show detailed information on the waiting page                           |
| `dynamic.theme`            | string   | No       | Server default  | Theme for the waiting page (e.g., `hacker-terminal`, `ghost`, `matrix`) |
| `dynamic.refreshFrequency` | duration | No       | Server default  | How often the waiting page checks if instances are ready                |
| `blocking.timeout` | duration | No       | -       | Maximum time to wait for instances to become ready |
| `ignoreUserAgent` | string   | No     | -        | User Agent that should be ignored (middleware sends `HTTP 200` to not wake up the container)

## Usage

The plugin can be configured in different ways depending on your deployment context:

- **[Dynamic Configuration](#dynamic-configuration)**: Universal approach using configuration files (works with all Traefik providers)
- **[Docker](#docker)**: Label-based configuration for Docker containers (requires Traefik v3.6.0+)
- **[Docker Swarm](#docker-swarm)**: Swarm-specific configuration with centralized middleware definition
- **[Kubernetes](#kubernetes)**: Kubernetes-native configuration using annotations and IngressRoute

Choose the section that matches your deployment environment below.

### Dynamic Configuration

> [!NOTE]
> This configuration method works with **all Traefik providers** (Docker, Swarm, Kubernetes, File, etc.).

Configure the plugin in your Traefik dynamic configuration:

```yaml
http:
  middlewares:
    my-sablier:
      plugin:
        sablier:
          sablierUrl: http://sablier:10000  # Sablier service URL (must be reachable from Traefik)
          group: my-app-group               # Group name for managing instances collectively
          sessionDuration: 1m               # Session duration before shutting down instances
          # Only one strategy can be used at a time
          # Declare either `dynamic` or `blocking`, not both
          ignoreUserAgent: curl             # (Optional) Ignore requests with User Agent header curl to not wake up container

          # Dynamic strategy: displays a waiting page
          dynamic:
            displayName: My Title       # (Optional) Display name (defaults to middleware name)
            showDetails: true           # (Optional) Show details for this middleware (defaults to Sablier server settings)
            theme: hacker-terminal      # (Optional) Theme for the waiting page
            refreshFrequency: 5s        # (Optional) Refresh frequency for the waiting page

          # Blocking strategy: waits for services to start (up to the timeout limit)
          # blocking: 
          #   timeout: 1m
```

<!-- TODO: Add usage example in a route -->

### Docker

<img src="./docs/assets/img/docker.svg" alt="docker" width="100" align="right" />

> [!IMPORTANT]
> Label-based configuration requires **Traefik v3.6.0 or higher**.
> This version introduced support for non-running containers ([PR #10645](https://github.com/traefik/traefik/pull/10645)), which is essential for Traefik to register your configuration while scaled to zero.

> [!TIP]
> For a complete working example, see the [docker example](./examples/docker/).

Add the following labels to the service you want to scale on demand:

```yaml
whoami:
  image: traefik/whoami:latest
  labels:
    # Enable Sablier for this container
    - sablier.enable=true
    - sablier.group=whoami

    # Configure the Sablier middleware
    - traefik.http.middlewares.whoami-sablier.plugin.sablier.group=whoami
    - traefik.http.middlewares.whoami-sablier.plugin.sablier.sablierUrl=http://sablier:10000
    - traefik.http.middlewares.whoami-sablier.plugin.sablier.sessionDuration=1m
    - traefik.http.middlewares.whoami-sablier.plugin.sablier.dynamic.displayName=Whoami Service

    # Allow Traefik to route to non-running containers
    - traefik.docker.allownonrunning=true

    # Define Traefik routing
    - traefik.http.routers.whoami.rule=Host(`whoami.localhost`)
    - traefik.http.routers.whoami.middlewares=whoami-sablier
```

### Docker Swarm

<img src="./docs/assets/img/docker_swarm.png" alt="docker_swarm" width="100" align="right" />

> [!CAUTION]
> When using Docker Swarm, the middleware configuration **must be defined on a service scanned by Traefik that is not scaled to zero**.
> 
> This is because Traefik evicts services from its pool when scaled to 0/0, making service-level labels unavailable. 
>
> Enabling `AllowEmptyServices ` is supposed to cover this use case but this is a known issue: **[AllowEmptyServices option does not work with the Swarm provider #12196
](https://github.com/traefik/traefik/issues/12196)**

> [!TIP]
> For a complete working example, see the [Docker Swarm E2E test](./e2e/docker_swarm/).

**1. Deploy Sablier with Middleware Configuration**

Deploy Sablier as a service and define all middleware configurations on it:

```yaml
sablier:
  image: sablierapp/sablier:1.10.1
  command:
    - start
    - --provider.name=swarm
  volumes:
    - '/var/run/docker.sock:/var/run/docker.sock'
  deploy:
    labels:
      - traefik.enable=true
      # Define the Sablier middleware here
      - traefik.http.middlewares.whoami-sablier.plugin.sablier.group=my-group
      - traefik.http.middlewares.whoami-sablier.plugin.sablier.sablierUrl=http://tasks.sablier:10000
      - traefik.http.middlewares.whoami-sablier.plugin.sablier.sessionDuration=1m
      - traefik.http.middlewares.whoami-sablier.plugin.sablier.dynamic.displayName=Whoami Service
      - traefik.http.services.sablier.loadbalancer.server.port=10000
```

**2. Configure Traefik with Swarm Provider**

Enable the Swarm provider with empty services support:

```yaml
traefik:
  image: traefik:v3.0.4
  command:
    - --experimental.plugins.sablier.modulename=github.com/sablierapp/sablier-traefik-plugin
    - --experimental.plugins.sablier.version=v1.0.0
    - --entryPoints.http.address=:80
    - --providers.swarm=true
    - --providers.swarm.refreshSeconds=1
  ports:
    - target: 80
      published: 8080
  volumes:
    - '/var/run/docker.sock:/var/run/docker.sock'
```

**3. Configure Your Service Labels**

Add labels to the service you want to scale on demand:

```yaml
whoami:
  image: acouvreur/whoami:v1.10.2
  deploy:
    replicas: 0
    labels:
      # Enable Sablier for this service
      - sablier.enable=true
      - sablier.group=my-group
      
      # Enable Traefik routing
      - traefik.enable=true
      
      # Use Swarm load balancer to prevent service eviction
      - traefik.docker.lbswarm=true
      
      # Define routing and attach the middleware
      - traefik.http.routers.whoami.rule=Host(`whoami.localhost`)
      - traefik.http.routers.whoami.middlewares=whoami-sablier@swarm
      - traefik.http.routers.whoami.service=whoami
      - traefik.http.services.whoami.loadbalancer.server.port=80
```

**⚠️ Limitations**

- The [blocking strategy](https://docs.sablier.app/strategies#blocking-strategy) cannot be used reliably due to how Traefik handles service eviction with empty pools
- Middleware configuration must be centralized on the Sablier service to remain accessible when services are scaled to 0

### Kubernetes

<img src="./docs/assets/img/kubernetes.png" alt="kubernetes" width="100" align="right" />

⚠️ **Limitations**

- Traefik evicts services from its pool when no endpoints are available. Enable [`allowEmptyServices`](https://doc.traefik.io/traefik/providers/kubernetes-ingress/#allowemptyservices) to prevent this.
- The blocking strategy is not yet supported due to Traefik's pod IP handling.

Refer to the [Kubernetes E2E test script](./e2e/kubernetes.sh) for a working example.

## Other Reverse Proxy Plugins

### Apache APISIX

<img src="./docs/assets/img/apacheapisix.png" alt="Apache APISIX" width="100" align="right" />

Sablier integrates with Apache APISIX through a Proxy-WASM plugin, enabling dynamic scaling for your services.

**Quick Start:**
1. Install the Sablier Proxy-WASM plugin
2. Configure APISIX routes with Sablier plugin settings
3. Define your scaling labels on target services

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Caddy

<img src="./docs/assets/img/caddy.png" alt="Caddy" width="100" align="right" />

Sablier provides a native Caddy module for seamless integration with Caddy v2.

**Quick Start:**
1. Build Caddy with the Sablier module using `xcaddy`
2. Add Sablier directives to your Caddyfile
3. Configure dynamic scaling rules

📚 **[Full Documentation](https://github.com/sablierapp/sablier-caddy-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-caddy-plugin)**

---

### Envoy

<img src="./docs/assets/img/envoy.png" alt="Envoy" width="100" align="right" />

Sablier integrates with Envoy Proxy through a Proxy-WASM plugin for high-performance dynamic scaling.

**Quick Start:**
1. Deploy the Sablier Proxy-WASM plugin
2. Configure Envoy HTTP filters
3. Set up scaling labels on your workloads

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Istio

<img src="./docs/assets/img/istio.png" alt="Istio" width="100" align="right" />

Sablier works with Istio service mesh using the Proxy-WASM plugin for intelligent traffic management.

**Quick Start:**
1. Install the Sablier Proxy-WASM plugin in your Istio mesh
2. Configure EnvoyFilter resources
3. Annotate your services with Sablier labels

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

---

### Nginx

<img src="./docs/assets/img/nginx.svg" alt="Nginx" width="100" align="right" />

Sablier integrates with Nginx through a WASM module, bringing dynamic scaling to your Nginx deployments.

**Quick Start:**
1. Build Nginx with WASM support
2. Load the Sablier Proxy-WASM plugin
3. Configure Nginx locations with Sablier directives

📚 **[Full Documentation](https://github.com/sablierapp/sablier-proxywasm-plugin)** | 💻 **[Plugin Repository](https://github.com/sablierapp/sablier-proxywasm-plugin)**

## Community

Join our Discord server for discussions and support:

[![Discord](https://img.shields.io/discord/1298488955947454464?logo=discord&logoColor=5865F2&cacheSeconds=1&link=http%3A%2F%2F)](https://discord.gg/Gpz5ha8WWx)

## Support

This project is maintained by a single developer in their free time. If you find Sablier useful, here are some ways you can show your support:

⭐ **Star the repository** - It helps others discover the project and motivates continued development

🤝 **Contribute** - Pull requests are always welcome! Whether it's:
- Bug fixes
- New features
- Documentation improvements
- Test coverage

📚 **Share your usage** - We'd love to see how you're using Sablier! Consider:
- Opening a discussion to share your setup
- Contributing examples of your deployment configurations
- Writing a blog post or tutorial

💬 **Engage with the community** - Ask questions, report issues, or help others in [discussions](https://github.com/sablierapp/sablier/discussions)

Every contribution, no matter how small, makes a difference and is greatly appreciated! 🙏

For detailed support options, see [SUPPORT.md](SUPPORT.md).