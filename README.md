# ecr-secret-operator

Kubernetes operator to manage rotation of AWS ECR docker-registry secrets

## Description

AWS policy on ECR is that the authentication token, once acquired is valid for 12 hours. This poses problems for clusters that are not AWS aware in that you cannot create an image pull secret for your ECR credential, as it will soon become invalid. This operator solves this problem by managing your ECR image pull secrets, updating them before they expire.

## Custom Resources

The operator provides a single custom resource which manages the lifetime of ECR image pull secrets.

```yaml
apiVersion: secrets.fireflycons.io/v1beta1
kind: ECRSecret
metadata:
  name: ecrsecret-sample
spec:
  registry: 0123456789012.dkr.ecr.us-east-1.amazonaws.com
  secretName: my-ecr-secret     # <- Optional

```

Where

|Property|Required|Description|
|--------|--------|-----------|
|`registry`|Yes     | ECR registry to manage secret for |
|`secretName`|No    | Optional name for generated Kubernetes secret. If omitted, secret will be named `<ECRSecret.name>-secret`

When a resource of the above type is deployed, the operator will create a Kubernetes secret in the same namespace with a name as defined by the above rules. The auth token in the Kubernetes secret will be rotated at least as frequently as specificed by the operator argument `--max-age`.

## Operator Command Line Arguments

```
  --config-file string
        The path to the configuration file containing AWS credentials
  --health-probe-bind-address string
        The address the probe endpoint binds to. (default ":8081")
  --kubeconfig string
        Paths to a kubeconfig. Only required if out-of-cluster.
  --leader-elect
        Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  --max-age duration
        The maximum age the secret can be before being rotated. (default 8h0m0s)
  --metrics-bind-address string
        The address the metric endpoint binds to. (default ":8080")
  --zap-devel
        Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). 
        Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
  --zap-encoder value
        Zap log encoding (one of 'json' or 'console')
  --zap-log-level value
        Zap Level to configure the verbosity of logging.
        Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  --zap-stacktrace-level value
        Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
  --zap-time-encoding value
        Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```

## Installing

A Helm chart is provided in [helmchart/ecr-secret-operator](.helmchart/ecr-secret-operator)


### Manager process values

| Key                                                                     | Value                                                |
|-------------------------------------------------------------------------|------------------------------------------------------|
| `ecrSecretOperatorControllerManagerDeployment.manager.image.repository` | Image repository.                                    |
| `ecrSecretOperatorControllerManagerDeployment.manager.image.tag`        | Image tag.                                           |
| `ecrSecretOperatorControllerManagerDeployment.manager.replicas`         | Number of operator replicas to run.                  |
| `ecrSecretOperatorControllerManagerDeployment.manager.args`             | List of command line arguments for operator process. |


### AWS Account configuration

You must configure at least one AWS account for the operator to use

In your custom values file, the key `AWS` contains one or more sub-keys where each sub-key is an AWS account ID. Beneath each sub-key is the access key and secret key to use with the account. Note that the IAM::User with which the keys are associated requires read access to ECR to authenticate and pull images. A sample CloudFormation for such a user can be found [here](./aws-infrastructure/CloudFormation.yaml).

```yaml
AWS:
  "0123456789012":
    accessKey: AKAIEXAMPLE
    secretKey: dskwr4EXAMPLE
```

The AWS information can also be inserted with helm `--set` arguments

```sh
helm install my-release helmcharts/ecr-secret-operator \
   --set AWS.0123456789012.accessKey=AKAIEXAMPLE \
   --set AWS.0123456789012.secretKey=dskwr4EXAMPLE
```
## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

