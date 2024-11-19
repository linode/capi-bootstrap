#!/bin/bash

DEFAULT_MANIFEST_DIR="/var/lib/kubeadm/manifests"
export KUBECONFIG=/etc/kubernetes/admin.conf
MANIFEST_DIR=${1:-$DEFAULT_MANIFEST_DIR}
#kubectl label nodes [[[ .ClusterName ]]]-bootstrap "node-role.kubernetes.io/control-plane=true" --overwrite
# install k3s/helm controller
#kubectl apply -f https://github.com/k3s-io/helm-controller/releases/download/v0.16.3/deploy-cluster-scoped.yaml --wait
# add tolerations for running on a control plane node since that is all that is running at this time
#kubectl patch deployment helm-controller --type=strategic -p '{
#  "spec": {
#    "template": {
#      "spec": {
#        "hostNetwork": true,
#        "tolerations": [
#          {
#            "key": "node-role.kubernetes.io/control-plane",
#            "effect": "NoSchedule"
#          },
#          {
#            "key": "CriticalAddonsOnly",
#            "operator": "Exists"
#          },
#          {
#            "key": "node.cloudprovider.kubernetes.io/uninitialized",
#            "value": "true",
#            "effect": "NoSchedule"
#          },
#          {
#            "key": "node.kubernetes.io/not-ready",
#            "operator": "Exists",
#            "effect": "NoSchedule"
#          }
#        ],
#        "volumes": [
#          {
#            "name": "kubeconfig",
#            "hostPath": {
#              "path": "/etc/kubernetes/admin.conf",
#              "type": "File"
#            }
#          }
#        ],
#        "containers": [
#          {
#            "name": "helm-controller",
#            "args": [
#              "-m",
#              "https://[[[ .ClusterEndpoint ]]]:6443",
#              "-k",
#              "/etc/kubernetes/admin.conf"
#            ],
#            "volumeMounts": [
#              {
#                "name": "kubeconfig",
#                "mountPath": "/etc/kubernetes/admin.conf",
#                "readOnly": true
#              }
#            ]
#          }
#        ]
#      }
#    }
#  }
#}'
# install all yaml files in the kubeadm manifests dir
bash /tmp/helm-install.sh
RETRY_INTERVAL=5
retry=true
while [ "${retry}" = true ]; do
    retry=false
    for file in "${MANIFEST_DIR}"/*.yaml; do
      if  [ ! -f "${file}.ts " ]; then
          echo "Applying manifest ${file}"e
          kubectl create -f "${file}"
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
kubectl patch machine [[[ .ClusterName ]]]-bootstrap --type=json -p "[{\"op\": \"add\", \"path\": \"/metadata/ownerReferences\", \"value\" : [{\"apiVersion\":\"controlplane.cluster.x-k8s.io/v1beta1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KubeadmControlPlane\",\"name\":\"[[[ .ClusterName ]]]-control-plane\",\"uid\":\"$(KUBECONFIG=/etc/kubernetes/admin.conf kubectl get KubeadmControlPlane [[[ .ClusterName ]]]-control-plane -ojsonpath='{.metadata.uid}')\"}]}]"
kubectl patch cluster [[[ .ClusterName ]]] --type=json -p '[{"op": "replace", "path": "/spec/controlPlaneRef/name", "value": "[[[ .ClusterName ]]]-control-plane"}]'
