# CAPI Bootstrap

## Building Requirements
Note: Using devbox will install necessary requirements to build/run this project, dependencies can be installed outside Devbox as well
* (Optional)[Devbox](https://www.jetify.com/devbox/)
  * Golang
  * clusterctl
  * kubectl

## Getting started
Note: if you are using devbox, enter into a shell using `devbox shell` before running these commands to use its integration.
1. Build `clusterct-bootstrap`
    ```shell
    make all
    ```
2. set environment variables
    ```shell
    # needed to create the bootstrap VM in the new cluster
    export LINODE_TOKEN=$GENERATED_LINODE_TOKEN
    # used for connecting to machines directly to get kubeconfig and other debug steps
    export AUTHORIZED_KEYS=$YOUR_PUBLIC_KEY
    # if you are not using devbox, add ./bin/ to your path so clusterctl will recognize it as a plugin 
    export PATH=$PATH:./bin/
    # set the name of your cluster
    export CLUSTER_NAME=test-cluster
    ```
3. Create a cluster

   _Note: If you are not using devbox make sure to run `export PATH=$PATH:./bin/`_
    ```shell
    clusterctl generate cluster $CLUSTER_NAME --control-plane-machine-count=3 --worker-machine-count=3   --kubernetes-version v1.29.4+k3s1     --infrastructure linode-linode:v0.3.1  --flavor k3s-vpcless > test-cluster-k3s.yaml
    clusterctl bootstrap cluster -m test-cluster-k3s.yaml
    # I0603 10:12:51.190936   70482 cluster.go:85] cluster name: test-cluster
    # I0603 10:12:51.814196   70482 cluster.go:116] Created NodeBalancer: test-cluster
    # I0603 10:12:53.447277   70482 cluster.go:165] Created Linode Instance: test-cluster-bootstrap
    # I0603 10:12:53.644074   70482 cluster.go:185] Created NodeBalancer Node: test-cluster-bootstrap
    # I0603 10:12:53.644124   70482 cluster.go:186] Bootstrap Node IP: <bootstrap IP>
    ```
4. Get kubeconfig for cluster
    ```shell
    clusterctl bootstrap get kubeconfig $CLUSTER_NAME --ssh  -i $PRIVATE_SSH_KEY_FILE --password $SSH_KEY_PASS > test-kubeconfig
    # I0603 10:29:23.547688   71898 get_kubeconfig.go:162] Connecting by SSH to <bootstrap IP>:22 using identify file $PRIVATE_SSH_KEY_FILE with username root and a password
    ```
5. Connect to cluster
    ```shell
    KUBECONFIG=./test-kubeconfig kubectl get nodes                                                                                                                                                                                                                            ✔  ▼  kind-tilt ⎈  impure  
    NAME                               STATUS   ROLES                       AGE   VERSION
    test-cluster-bootstrap             Ready    control-plane,etcd,master   24m   v1.29.5
    test-cluster-control-plane-7pgmx   Ready    control-plane,etcd,master   17m   v1.29.5
    test-cluster-control-plane-gnq5g   Ready    control-plane,etcd,master   19m   v1.29.5
    ```
6. Delete cluster
    ```shell
    clusterctl bootstrap delete --force test-cluster                                                                                                                                                                                                                           ✔  4s   ▼  impure  
    # I0603 10:42:33.972689   73227 delete.go:67] Will delete instances:
    # I0603 10:42:33.972940   73227 delete.go:69]   Label: test-cluster-bootstrap, ID: 2345534
    # I0603 10:42:33.972952   73227 delete.go:69]   Label: test-cluster-control-plane-gnq5g, ID: 23452345
    # I0603 10:42:33.972960   73227 delete.go:69]   Label: test-cluster-control-plane-7pgmx, ID: 52764566
    # I0603 10:42:34.080395   73227 delete.go:78] Will delete NodeBalancer:
    # I0603 10:42:34.080429   73227 delete.go:79]   Label: test-cluster, ID: 7856756
    # I0603 10:42:34.080443   73227 delete.go:97] Deleting resources:
    # I0603 10:42:34.574166   73227 delete.go:103]   Deleted Instance test-cluster-bootstrap
    # I0603 10:42:35.010239   73227 delete.go:103]   Deleted Instance test-cluster-control-plane-gnq5g
    # I0603 10:42:35.503298   73227 delete.go:103]   Deleted Instance test-cluster-control-plane-7pgmx
    # I0603 10:42:35.730360   73227 delete.go:110]   Deleted NodeBalancer test-cluster
    ```
