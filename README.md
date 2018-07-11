Terraform Provider Tar
======================

Usage
---------------------

install provider binary

```sh
go get -u github.com/Zhuvikin/terraform-provider-tar
```

Add to ~/.terraformrc tar provider

```sh
providers {
  tar = "/$GOPATH/bin/terraform-provider-tar"
}
```

Use tar template data source in your terraform code
```sh
data "tar_template" "configs_dir" {
  source_dir = "${path.module}/resources/master/kubernetes"
  vars {
    k8s_api_secure_port = "6443"
  }
}

resource "null_resource" "kubernetes-folder-upload" {
  count = "${var.masters_number}"
  triggers {
    content = "${data.tar_template.configs_dir.rendered}"
  }
  connection {
    host = "..."
    type = "ssh"
    user = "..."
    password = "..."
  }
  provisioner "file" {
    content = "${data.tar_template.configs_dir.rendered}"
    destination = "/etc/kubernetes.tar"
  }
  provisioner "remote-exec" {
    inline = [
      "tar xvf /etc/kubernetes.tar"
    ]
  }
}

```