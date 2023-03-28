# aws-infrastructure

This directory contains a CloudFormation template that creates an `IAM::User` with the minimum permission required to manage ECR docker-registry secrets in your cluster. The access keys of this account should be specified in `config.toml`. The table header is your AWS account ID.

Keys for multiple accounts may be added to `config.toml` by adding additional tables. Create the `IAM::User` infrastructure in all accounts the operator requires access to.

```toml
[123456789012]
access_key = "AKIAEXAMPLE"
secret_key = "aasqrd2323EXAMPLE"
```
