# Running microservices-demo on EKS (kubernetes on AWS)

This is not a full step-through; each of these steps are their own undertaking.
Here's what I had to do:

## AWS authentication

1. Have an AWS (sub)account, where I can log in to the console as root.
2. In IAM (search for the AWS service), create a new user with admin access. Set up 2FA for the console, and log in as this user. (This is probably not necessary but it felt better than operating as the root user.)
3. In IAM, get that user's Access Key ID and Secret Key. (I had to create a new one to get the secret.)
4. On my computer, install the AWS CLI.
5. Use `aws configure` to log in as my new user.

Use `aws sts get-caller-identity` to check the results.
It should show your account ID.

## Get a cluster

6. In the AWS Console, create an EKS cluster

Search for the EKS service. Create a new cluster. I picked all the defaults, except for adding a tag called microservices-demo. Even then, it took a lot of steps; it asked me to create some new security roles to put on components of the cluster. There were instructions.

consider naming it microservices-demo.

Note: only you, personally, your AWS user, will have access to the cluster. Even if other people have an admin role. Ugh.

7. Confirm that the cluster exists and I can access it

on my computer,
`aws eks list-clusters`

It should show the new cluster.

Note: no one else will be able to ask it. If you have another person to share with, you can make them an IAM user, and then:
`eksctl create iamidentitymapping --cluster microservices-demo --region=us-east-1 --arn arn:aws:iam::131312313131:user/yourfriend --group system:masters --username yourfriend`

where the ARN here is their result from `aws sts get-caller-identity` and their username is something, it doesn't seem to matter.

Note that you had to [install](https://docs.aws.amazon.com/eks/latest/userguide/eksctl.html) `eksctl` for this.

8. configure kubectl on my computer.

Install kubectl if I haven't already; and then:

`aws eks update-kubeconfig --region us-east-1 --name microservices-demotron`

note:

I got "Cluster status is CREATING" and then it didn't work... maybe the cluster needs to be up first?

check its status:

`aws eks describe-cluster --name microservices-demo`

Wait until it is ACTIVE.

end note.

Check whether it works:

`kubectl config get-contexts`

This should show one or more contexts (user+cluster), and the \* should be near the new one.

(to get back to a different one, it's `kubctl config use-context [name]`)

`kubectl get svc`

This should show a ClusterIP service.

`kubectl get nodes`

No nodes were listed.

## Get some nodes

7. Add some nodes to the cluster. When I looked at my cluster in the AWS Console (after searching for EKS), it gave me a popup about adding nodes.

I made an EKS node group for this. It was a whole process, even sticking with the defaults. It wanted a new security group again, and this time linked to a set of instructions that told me what roles to manually add to the new security group.

Later I found that I needed 3 nodes in the node group.

See the node group:

`aws eks list-nodegroups --cluster-name microservices-demo`

8. See some nodes

## Now spin up the collector

We spin up the collector using `helm`. Follow the "OpenTelemetry Collector Helm Chart" step in the README.

`kubectl get svc` should show the collector, after this works.

(note: if you forget to provide it your real API key, as I did the first time, then it won't recreate until you delete it. That is `helm delete opentelemetry-collector`.)

## Get a container registry

Skaffold is going to build the containers and put them into the local docker image repository, but k8s can't get them from there. We have to push them up somewhere.

I used ECR for this, AWS's container registry. This was hard. Here's what I did:

1. In the AWS Console, I created one container repository, called (specifically) `checkoutservice`

2. Test that I can access it: `aws ecr describe-repositories`

That gives a repository URI. It looks like: `313131313131.dkr.ecr.us-east-1.amazonaws.com`

You'll need this in the next step.

3. Configure skaffold to push to this repository.

Install skaffold if you haven't yet. Try `skaffold run` in this repo, and see that it fails on pushing an image. Time to configure it.

You'll need your EKS cluster's resource identifier (ARN). Get it with

`aws eks describe-cluster --name microservices-demo`

(If microservices-demo is not the name of your cluster, you can find its name with `aws eks list-clusters`)

Put this in for `kube-context` in the file `~/.skaffold/config`. Add a suffix so the containers you push will be specific to this cluster.
Here's what mine looks like:

```
kubeContexts:
- kube-context: arn:aws:eks:us-east-1:414141414141:cluster/microservices-demo
  default-repo: 414141414141.dkr.ecr.us-east-1.amazonaws.com/microservices-demo
```

Then put your ECS repository URI in for `default-repo`.

_cleanup_: if you decide to wipe out these repos: `grep image: skaffold.yaml | cut -d : -f 2 | sed 's/^ /microservices-demo\//' | xargs -L 1 aws ecr delete-repository --region us-east-1 --repository-name`

4. Log in to the container registry.

Here's the spell:

`aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 313131313131.dkr.ecr.us-east-1.amazonaws.com`

Put your region in for us-east-1, and your account ID for 313131313131.

This login doesn't last forever!! You'll need to do this again tomorrow, or after some number of hours.

5. Create the other repositories you are going to need.

We need a repository per image name. See the different images in `skaffold.yaml`. You can create the repositories in the AWS console, or do this at the command line (mac or linux):

`grep image: skaffold.yaml | cut -d : -f 2 | sed 's/^ /microservices-demo\//' | xargs -L 1 aws ecr create-repository --region us-east-1 --repository-name`

## Build and deploy

Now try `skaffold run` in this repo. Maybe it can push them!

... my next failure is in trying to get the honeycomb secret. Clearly I need to put an API key somewhere for these services.
