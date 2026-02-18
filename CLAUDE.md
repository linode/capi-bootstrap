# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build          # Build clusterctl-bootstrap binary to ./bin/
make test           # Run tests with race detector and coverage
make lint           # Run golangci-lint
make fmt            # Format code
make generate       # Generate mocks (requires mockgen)
make all            # Clean, fmt, test, vet, build
```

Run a single test:
```bash
go test ./providers/infrastructure/linode -run TestSpecificName
```

## Architecture

This is a Kubernetes Cluster API (CAPI) bootstrap orchestrator using a **three-layer provider plugin architecture**:

```
cmd/                          # Cobra CLI commands (entry points)
providers/
├── infrastructure/           # Cloud resource provisioning (Linode)
├── backend/                  # State storage (S3, GitHub)
└── controlplane/             # K8s control plane setup (K3s)
cloudinit/                    # Cloud-init YAML generation
state/                        # Cluster state serialization
yaml/                         # Manifest parsing and construction
types/                        # Shared data structures
```

### Provider Interfaces

Each provider type implements a specific interface in `providers/{type}/types.go`:

- **Infrastructure Provider**: `PreCmd()`, `PreDeploy()`, `Deploy()`, `PostDeploy()`, `Delete()` - provisions cloud instances, load balancers, networks
- **Backend Provider**: `Read()`, `WriteConfig()`, `WriteFiles()`, `Delete()`, `ListClusters()` - persists/retrieves cluster state
- **Control Plane Provider**: generates CAPI manifests, init scripts, handles certificates and kubeconfig

Providers are instantiated via factory functions:
- `infrastructure.NewProvider(kind string)`
- `backend.NewProvider(name string)`
- `controlplane.NewProvider(kind string)`

### Key Workflow

1. `cmd/cluster.go:runBootstrapCluster()` orchestrates cluster creation
2. Parses cluster manifest YAML → instantiates providers based on Kind
3. Calls infrastructure provider lifecycle: PreDeploy → Deploy → PostDeploy
4. `cloudinit.GenerateCloudInit()` combines artifacts from all providers
5. Backend provider persists state (kubeconfig with Extensions containing cluster metadata)

### Configuration

- Config file: `$XDG_CONFIG_HOME/cluster-api/bootstrap.yaml`
- Supports profiles for different environments
- Precedence: CLI args > profile > defaults

## Required Environment Variables

See README.md for full provider configuration. Key variables:
- `LINODE_TOKEN` - Linode API token
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `S3_BUCKET` - S3 backend
- `GITHUB_TOKEN` - GitHub backend
