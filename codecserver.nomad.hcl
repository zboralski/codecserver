job "codecserver" {
  datacenters = ["${datacenter}"]
  group "codecserver" {
    network {
      mode = "host"

      port "codecserver" {
        static = ${port}
        to     = ${port}
      }
    }

    service {
      provider = "nomad"
      name     = "codecserver"
      port     = "codecserver"

      check {
        type     = "http"
        path     = "/health"
        port     = "codecserver"
        interval = "10s"
        timeout  = "1m"
      } 
    }

    vault {
      policies = ["nomad-codecserver"]
    }

    task "codecserver" {
      driver = "docker"
      config {
        image = "${artifact.image}:${artifact.tag}"
	      ports = ["codecserver"]
      }

      env {
        %{ for k,v in entrypoint.env ~}
        ${k} = "${v}"
        %{ endfor ~}

        PORT = "$${NOMAD_PORT_codecserver}"
        TLS_CERT_FILE = "$${NOMAD_SECRETS_DIR}/server.pem"
        TLS_KEY_FILE = "$${NOMAD_SECRETS_DIR}/server-key.pem"
        VAULT_ADDR = "${vault_addr}"
        WAYPOINT_CEB_DISABLE_EXEC = "1"
      }

      template {
        data        = <<EOH
{{ with secret "pki_int/issue/codecserver" "common_name=${hostname}" "format=pem" }}{{ .Data.certificate }}{{ end }}
        EOH
        destination = "$${NOMAD_SECRETS_DIR}/server.pem"
        change_mode = "noop"
      }      

      template {
        data        = <<EOH
{{ with secret "pki_int/issue/codecserver" "common_name=${hostname}" "format=pem" }}{{ .Data.private_key }}{{ end }}
        EOH
        destination = "$${NOMAD_SECRETS_DIR}/server-key.pem"
        change_mode = "restart"
        perms       = "0400"
      }   
    }
  }
}