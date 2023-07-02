# Pre OOM Killer

Pre OOM Killer evict the specified pod before it is OOMKilled.

Pre-oom-killer is based on preoomkiller-controller created by zapier and modified to evict by memory usage.  
ref: https://github.com/zapier/preoomkiller-controller

## Getting started

### Install with helm

```bash
$ helm repo add pre-oom-killer https://syossan27.github.io/pre-oom-killer/v0.1.0
$ helm install pre-oom-killer pre-oom-killer/pre-oom-killer
```

## Structure

Using client-go, if the container of a pod to which the specified labels and annotations are attached exceeds the specified memory usage, the pod is marked as evict.  
Pods marked as evict are automatically deleted by k8s and restarted by auto-healing.