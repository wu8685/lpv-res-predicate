This is a kubernetes scheduler HTTP extender callee which is used to reserve resources (CPU and Memory) for pods using local persistent volumes.

Indicate the quantiiesy of reserved resources by annotation on persistent volume:

```
"reserved-cpu": "100m"
"reserved-mem": "100M"
```

## Plug in Kubernetes

Create scheduler policy configmap:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: scheduler-policy
  namespace: kube-system
data:
  policy.cfg: |
    {
      "ExtenderConfigs": [
        {
          "URLPrefix": "http://192.168.31.12:8089/filter",
          "FilterVerb": "lpvReservedResource",
          "EnableHttps": false,
          "NodeCacheCapable": true
        }
      ]
    }
```

Indicate scheduler by argument:

```
# namespace defaults kube-system
--policy-configmap=scheduler-policy
```

if using minikube, the scheduler RBAC needs to be updated by adding this rule:

```
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - list
  - get
```