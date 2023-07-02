# Pre OOM Killer

## Notice

Pre OOM Killer is under development!   
The service I am working on is working fine, but please be careful with it.

I am also open to your requests and ideas. I can't say that we will adopt all of them, but if there are good ideas, we will adopt them.

## Overview

Pre OOM Killer evict the specified pod before it is OOMKilled.

Pre-oom-killer is based on preoomkiller-controller created by zapier and modified to evict by memory usage.  
ref: https://github.com/zapier/preoomkiller-controller

## Getting started

### Install with helm

```bash
$ helm repo add pre-oom-killer https://syossan27.github.io/pre-oom-killer/v0.1.0
$ helm install pre-oom-killer pre-oom-killer/pre-oom-killer
```

### Set Label and Annotation

The "pre-oom-killer" is executed based on the labels and annotations described in the pod's metadata.  
The following is an example using yaml to deploy nginx.

 - Deployment 
```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      labels:
        app: nginx
        pre-oom-killer: enabled
      annotations:
        pre-oom-killer.v1alpha1.k8s.io/target-container-name: "nginx"
        pre-oom-killer.v1alpha1.k8s.io/memory-usage-threshold: "60"
```

 - Pod
```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: nginx
    pre-oom-killer: enabled
  annotations:
    pre-oom-killer.v1alpha1.k8s.io/target-container-name: "nginx"
    pre-oom-killer.v1alpha1.k8s.io/memory-usage-threshold: "60"
``` 

The labels and annotations should include the following three fields.

 - labels.pre-oom-killer : Enable pre-oom-killer, which is enabled if enabled and disabled otherwise or if there are no labels
 - annotations.pre-oom-killer.v1alpha1.k8s.io/target-container-name : Specify the name of the container to be monitored for memory usage
 - pre-oom-killer.v1alpha1.k8s.io/memory-usage-threshold : Memory usage threshold to evict pods

## Structure

Using client-go, if the container of a pod to which the specified labels and annotations are attached exceeds the specified memory usage, the pod is marked as evict.  
Pods marked as evict are automatically deleted by k8s and restarted by auto-healing.