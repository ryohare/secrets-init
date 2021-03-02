# secrets-init

`secrets-init` is a minimalistic init system designed to run inside a container where a shell script file with environment variables can be sourced to be consumed by the main application. It was developed to provide secrets to Kubernetes containers detailed [here](https://github.com/ryohare/secrets-sidecar-eks-poc). It resolves environment variables which hold arn values pointing to AWS Secrets Manager or AWS Parameter store. These values are resolved, put into the shell script and exported as the Key, Value pair as they appear in AWS. It supports the following secrets storage engines:

- [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/)
- [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html)

## Basic Usage
```bash
secrets-init <script-to-generate>
```

## Integration with AWS Secrets Manager

User can put AWS secret ARN as environment variable value. The `secrets-init` will resolve any environment value, using specified ARN, to referenced secret value and set an environment variable whos name maps to the name associated with the value in AWS. It will map the AWS Parameter Store name to the value in the source file.

```sh
# environment variable passed to `secrets-init`
MY_DB_PASSWORD=arn:aws:secretsmanager:$AWS_REGION:$AWS_ACCOUNT_ID:secret:mydbpassword-cdma3

secrets-init env.sh

cat env.sh
#!/bin/sh
export mydbpassword=very-secret-password
```

## Integration with AWS Systems Manager Parameter Store

It is possible to use AWS Systems Manager Parameter Store to store application parameters and secrets.

User can put AWS Parameter Store ARN as environment variable value. The `secrets-init` will resolve any environment value, using specified ARN, to referenced parameter value. It will map the AWS Parameter Store name to the value in the source file.

```sh
# environment variable passed to `secrets-init`
MY_API_KEY=arn:aws:ssm:$AWS_REGION:$AWS_ACCOUNT_ID:parameter/api/key
# OR versioned parameter
MY_API_KEY=arn:aws:ssm:$AWS_REGION:$AWS_ACCOUNT_ID:parameter/api/key:$VERSION

secrets-init sourcefile.sh

cat sourcefile.sh
#!/bin/sh
export api=key-123456789
```

## Building
### Local
Local building requires go >= 11 installed. To build locally, run `make` and the binary will go to `.bin/secrets-init`
```bash
make

.bin/secrets-init -v
```
### Docker
Docker can be used to build and to run the binary. Run `make docker` to make the docker image.
```bash
make docker

docker run -it --rm secrets-init-source
```

## Requirement

### AWS

In order to resolve AWS secrets from AWS Secrets Manager and Parameter Store, `secrets-init` should run under IAM role that has permission to access desired secrets.

This can be achieved by assigning IAM Role to Kubernetes Pod or ECS Task. It's possible to assign IAM Role to EC2 instance, where container is running, but this option is less secure.