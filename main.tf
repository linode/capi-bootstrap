terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "2.20.1"
    }
  }
}

provider "linode" {
}

resource "linode_nodebalancer" "capi-bootstrap" {
  label  = "capi-bootstrap"
  region = var.region
  tags   = ["capi-bootstrap"]
}

resource "linode_nodebalancer_config" "capi-bootstrap-api-server" {
  nodebalancer_id = linode_nodebalancer.capi-bootstrap.id
  port            = 6443
  protocol        = "tcp"
  algorithm       = "roundrobin"
  check           = "connection"
}

resource "linode_nodebalancer_node" "capi-bootstrap-node" {
  nodebalancer_id = linode_nodebalancer.capi-bootstrap.id
  config_id       = linode_nodebalancer_config.capi-bootstrap-api-server.id
  address         = "${linode_instance.bootstrap.private_ip_address}:6443"
  label           = "test-k3s"
  weight          = 100

  lifecycle {
    replace_triggered_by = [linode_instance.bootstrap.id]
  }
}

variable "linode_token" {
  type    = string
  default = ""
}

variable "authorized_keys" {
  type    = list(string)
  default = []
}

variable "region" {
  type    = string
  default = "us-mia"
}

resource "linode_instance" "bootstrap" {
  label           = "capi-bootstrap"
  image           = "linode/debian11"
  region          = var.region
  type            = "g6-standard-6"
  authorized_keys = var.authorized_keys
  root_pass       = "AkamaiPass123@1"
  metadata {
    user_data = base64encode(templatefile("cloud-config.yaml", {
      LINODE_TOKEN    = var.linode_token,
      AUTHORIZED_KEYS = jsonencode(var.authorized_keys),
      NB_IP           = linode_nodebalancer.capi-bootstrap.ipv4,
      NB_PORT         = linode_nodebalancer_config.capi-bootstrap-api-server.port,
      NB_ID           = linode_nodebalancer.capi-bootstrap.id,
    NB_CONFIG_ID = linode_nodebalancer_config.capi-bootstrap-api-server.id }))
  }

  tags       = ["capi-bootstrap"]
  private_ip = true
}

output "ssh" {
  value = "ssh root@${linode_instance.bootstrap.ip_address}"
}
output "api-server" {
  value = "https://${linode_nodebalancer.capi-bootstrap.ipv4}:6443"
}
