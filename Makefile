VERSION :=

helm-repo-add: __require_VERSION
	helm repo add pre-oom-killer https://syossan27.github.io/pre-oom-killer/${VERSION}

.PHONY: __require_VERSION
__require_VERSION:
ifndef VERSION
	$(error VERSION is not defined; you must specify VERSION like $$ make VERSION=v0.1.0 helm-repo-add)
endif