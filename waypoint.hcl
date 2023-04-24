project = "codecserver"

app "codecserver" {
  build {
    use "docker" {
      buildkit = true
      platform = "linux/amd64"
    }

    registry {
      use "docker" {
        image = "belua/codecserver"
        tag   = "1"
      }
    }
  }

  deploy {
    use "nomad-jobspec" {
      jobspec = templatefile("${path.app}/codecserver.nomad.hcl", {
        hostname    = var.hostname
        datacenter  = var.datacenter
        port        = var.port
        vault_addr  = var.vault_addr
      })
    }
  }

  release {}

  url {
    auto_hostname = false
  }
}

variable "hostname" {
  type    = string
  default = "localhost"
}

variable "datacenter" {
  type    = string
  default = "dc1"
}

variable "port" {
  type    = number
  default = 8081
  env     = ["PORT"]
}

variable "vault_addr" {
  type    = string
  default = "http://localhost:8200"
  env     = ["VAULT_ADDR"]
}