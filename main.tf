terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "2.20.1"
    }
  }
}

provider "linode" {}

resource "linode_instance" "bootstrap" {
  label           = "capi-bootstrap"
  image           = "linode/debian11"
  region          = "us-mia"
  type            = "g6-standard-6"
  authorized_keys = ["ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDp8NPNVd9U5h6QNxiA8uZolFyhAadSGaIxAiysQTCLbsNLMJRlyYWTWzymW4xoVkD1z1TIvNC8OrCeKu3AcBl9PKIOnCXIuwI6fRjzP5mmGqF6bec2TbMhg/f8FrBmYL4lXrk1s4oBxo1GO6gfuQe9lMFJbu2FOS4JdBGMJsH/ttFxg0NNzYgwzmJ9RXo4caCbdn4Tucr7kEDlHsWiI4FfMPlxz6Z91bGyZ1ZSjH/YcrveN6DIKtAh3/xruxuIa09bUJTJKRk+ctHCnpF7RMN+wnfRzh6Kn+32bZEgN5AX2qaS6KaxhprLi6XBVI1jBtVzQsDxWXMM5Oi3k5DZW3ckATO92E8AMyQurSzUC/rFpZf9amkCuqGi6twLblkQklmGZSp4MICynzVPEh3xzUewafki6S5fNhl/85DYah+0BmGtN3tYd4w1giBphHdYZrL91dLdZ2Wdcw/nTSAiJDLFv2TeMhf1V8cJYMrkwmN7x6mpaQznDogFeb7F2haPzYB3W+oSHPQN2SeMVG+AeaKJgb+22N6JdsLbbAV3UM3caVezgcMC6c4A2A49s1BOWg87eCderDpwrswdXXKhPxv0Imko0WBJ9PJnPkXPxIBLa2uavvdGu0ShdZd6MkMIQ72Jm9xPnzSrrOMbeQPKIxcNFIz3JWKtS5s4gPld7MraYQ== luther.monson@gmail.com"]
  root_pass       = "AkamaiPass123@1"
  metadata {
    user_data = base64encode(file("cloud-config.yaml"))
  }

  tags       = ["capi-bootstrap"]
  private_ip = true
}

output "ssh" {
  value = "ssh root@${linode_instance.bootstrap.ip_address}"
}