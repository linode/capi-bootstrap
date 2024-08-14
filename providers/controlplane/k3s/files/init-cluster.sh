#!/bin/bash
sed -i "s/127.0.0.1/[[[ .ClusterEndpoint ]]]/" /etc/rancher/k3s/k3s.yaml
until k3s kubectl get -f /var/lib/rancher/k3s/server/manifests/capi-manifests.yaml; do sleep 10; done
rm /var/lib/rancher/k3s/server/manifests/capi-manifests.yaml
k3s kubectl patch machine [[[ .ClusterName ]]]-bootstrap --type=json -p "[{\"op\": \"add\", \"path\": \"/metadata/ownerReferences\", \"value\" : [{\"apiVersion\":\"controlplane.cluster.x-k8s.io/v1beta1\",\"blockOwnerDeletion\":true,\"controller\":true,\"kind\":\"KThreesControlPlane\",\"name\":\"[[[ .ClusterName ]]]-control-plane\",\"uid\":\"$(k3s kubectl get KThreesControlPlane [[[ .ClusterName ]]]-control-plane -ojsonpath='{.metadata.uid}')\"}]}]"
k3s kubectl patch cluster [[[ .ClusterName ]]] --type=json -p '[{"op": "replace", "path": "/spec/controlPlaneRef/name", "value": "[[[ .ClusterName ]]]-control-plane"}]'
