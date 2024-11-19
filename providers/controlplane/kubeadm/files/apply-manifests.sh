#!/bin/sh

# install k3s/helm controller

# install all yaml files in the kubeadm manifests dir
DEFAULT_MANIFEST_DIR="/var/lib/kubeadm/manifests/"
KUBECONFIG=/etc/kubernetes/admin.conf
MANIFEST_DIR=${1:-$DEFAULT_MANIFEST_DIR}
RETRY_INTERVAL=5
retry=true
while [ "${retry}" = true ]; do
    retry=false
    for file in "${MANIFEST_DIR}"/*.yaml; do
      if  [ ! -f "${file}.ts " ]; then
          echo "Applying manifest ${file}"
          kubectl apply -f "${file}"
          if kubectl get -f "${file}"; then
            touch "${file}.ts";
          else
            retry=true
          fi
      fi
    done
    echo "missing resources, retrying apply in ${RETRY_INTERVAL}s"
    sleep $RETRY_INTERVAL
done
