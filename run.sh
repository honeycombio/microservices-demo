#!/bin/zsh

# This is meant to do a skaffold run using a remote repository without forcing a rebuild each time
# We do this by:
# 1. switch to the local context (which uses a local repository)
# 2. skaffold build
# 3. switch back to the original context
# 4. skaffold run


# change this to match your default local context
default_local_context="docker-desktop"

cur_context=$(kubectl config current-context)

kubectl config use-context "$default_local_context"
skaffold build
kubectl config use-context "$cur_context"
skaffold run
