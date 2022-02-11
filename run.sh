#!/bin/zsh

cur_context=$(kubectl config current-context)

kubectl config use-context docker-desktop
skaffold build
kubectl config use-context "$cur_context"
skaffold run
