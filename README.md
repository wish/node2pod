# node2pod

A dead simple controller with one job: copy certain labels on kubernetes nodes to all of the pods
they are running.

## Example deployment

This deployment will project the `topology.kubernetes.io/region` and `topology.kubernetes.io/zone`
labels from nodes to pods. See `deploy/` for complete resources, including RBAC.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: node2pod
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: node2pod
  template:
    metadata:
      labels:
        app: node2pod
    spec:
      containers:
      - command:
        - /bin/node2pod
        - --label=topology.kubernetes.io/region
        - --label=topology.kubernetes.io/zone
        image: quay.io/wish/node2pod:main
        name: node2pod
```
