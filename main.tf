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
  label = "capi-bootstrap"
  region = "us-mia"
  tags       = ["capi-bootstrap"]
}

resource "linode_nodebalancer_config" "capi-bootstrap-api-server" {
  nodebalancer_id = linode_nodebalancer.capi-bootstrap.id
  port = 6443
  protocol = "tcp"
  algorithm = "roundrobin"
  check = "connection"
}

resource "linode_nodebalancer_node" "capi-bootstrap-node" {
  nodebalancer_id = linode_nodebalancer.capi-bootstrap.id
  config_id       = linode_nodebalancer_config.capi-bootstrap-api-server.id
  address         = "${linode_instance.bootstrap.private_ip_address}:6443"
  label           = "bootstrap-node"
  weight          = 100

  lifecycle {
    replace_triggered_by = [linode_instance.bootstrap.id]
  }
}


resource "linode_instance" "bootstrap" {
  label           = "capi-bootstrap"
  image           = "linode/debian11"
  region          = "us-mia"
  type            = "g6-standard-6"
  authorized_keys = []
  root_pass       = "AkamaiPass123@1"
  metadata {
    user_data = base64encode(templatefile("cloud-config.yaml", { NB_IP = linode_nodebalancer.capi-bootstrap.ipv4 }))
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
