#!/bin/bash
sed -i "s/127.0.0.1/{{{ .Linode.NodeBalancerIP }}}/" /etc/rancher/k3s/k3s.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-kubeconfig --type=cluster.x-k8s.io/secret --from-file=value=/etc/rancher/k3s/k3s.yaml --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-kubeconfig.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-ca --type=cluster.x-k8s.io/secret --from-file=tls.crt=/var/lib/rancher/k3s/server/tls/server-ca.crt --from-file=tls.key=/var/lib/rancher/k3s/server/tls/server-ca.key --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-ca.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-cca --type=cluster.x-k8s.io/secret --from-file=tls.crt=/var/lib/rancher/k3s/server/tls/client-ca.crt --from-file=tls.key=/var/lib/rancher/k3s/server/tls/client-ca.key --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-cca.yaml
k3s kubectl create secret generic {{{ .ClusterName }}}-token --type=cluster.x-k8s.io/secret --from-file=value=/var/lib/rancher/k3s/server/token --dry-run=client -oyaml > /var/lib/rancher/k3s/server/manifests/cluster-token.yaml
until k3s kubectl get secret {{{ .ClusterName }}}-ca {{{ .ClusterName }}}-cca {{{ .ClusterName }}}-kubeconfig {{{ .ClusterName }}}-token; do sleep 5; done
k3s kubectl label secret {{{ .ClusterName }}}-kubeconfig {{{ .ClusterName }}}-ca {{{ .ClusterName }}}-cca {{{ .ClusterName }}}-token "cluster.x-k8s.io/cluster-name"="{{{ .ClusterName }}}"
until k3s kubectl get kthreescontrolplane {{{ .ClusterName }}}-control-plane; do sleep 10; done
k3s kubectl patch machine {{{ .ClusterName }}}-bootstrap --type=json -p "[{\"op\": \"add\", \"path\": \"/metadata/ownerReferences\", \"value\" : [{\"apiVersion\":\"controlplane.cluster.x-k8s.io/v1beta1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KThreesControlPlane\",\"name\":\"{{{ .ClusterName }}}-control-plane\",\"uid\":\"$(k3s kubectl get KThreesControlPlane {{{ .ClusterName }}}-control-plane -ojsonpath='{.metadata.uid}')\"}]}]"
sleep 15
k3s kubectl patch cluster {{{ .ClusterName }}} --type=json -p '[{"op": "replace", "path": "/spec/controlPlaneRef/name", "value": "{{{ .ClusterName }}}-control-plane"}]'
