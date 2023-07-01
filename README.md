# Pre OOM Killer

Pre OOM Killer evict the specified pod before it is OOMKilled.

Pre-oom-killer is based on preoomkiller-controller created by zapier and modified to evict by memory usage.  
ref: https://github.com/zapier/preoomkiller-controller

## Structure

Using client-go, if the container of a pod to which the specified labels and annotations are attached exceeds the specified memory usage, the pod is marked as evict.  
Pods marked as evict are automatically deleted by k8s and restarted by auto-healing.