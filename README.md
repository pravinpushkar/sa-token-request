# ServiceAccount TokenRequest
Create a kubernetes secret containing the service account token.

## Running the code locally

1. Point the `KUBECONFIG` environment variable to the kubeconfig file you want to use.
2. Run below command to run the code locally.
```bash
go run main.go
```

## Running the code in a container
1. Create a service account, role, and role binding in the namespace you want to run the code in and provide necessary permissions.
2. Run below code to build the docker image.

```bash
export HUB=<hub>
make docker-build
```
3. Create a deployment file to deploy the image to the cluster.


