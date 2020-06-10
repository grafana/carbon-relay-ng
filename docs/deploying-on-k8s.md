# Deploying Carbon-Relay-Ng on Kubernetes

We support two methods of deploying Carbon-Relay-Ng onto Kubernetes, 
via Ksonnet/Tanka and by directly applying the provided `.yaml` files.

# Ksonnet/Tanka

In this repository in `/ksonnet/lib/carbon-relay-ng` we provide a mixin which can be used in 
ksonnet to create a simple carbon-relay-ng deployment. 
Note that the mixin depends on the [ksonnet-util library](https://github.com/grafana/jsonnet-libs/tree/master/ksonnet-util).

Using the jsonnet bundler and ksonnet, a simple carbon-relay-ng deployment can be created like shown here:

```
# if jb isn't already initialized
jb init

# install the carbon-relay-ng mixin,
# this will also install the dependency github.com/grafana/jsonnet-libs/ksonnet-util
jb install 'github.com/grafana/carbon-relay-ng/ksonnet/lib/carbon-relay-ng@document_k8s_deployment'

# note that the ksonnet-util library depends on the file 'k.libsonnet' being available in
# the jsonnet search path, if you don't already have that then it needs to be made available
jb install github.com/ksonnet/ksonnet-lib/ksonnet.beta.4
echo "import 'ksonnet.beta.4/k.libsonnet'" > $JSONNET_PATH/k.libsonnet

# now you can use the mixin in ksonnet,
# this is how an example env with configuration could look like
$ cat env/main.jsonnet
local k = import 'ksonnet-util/kausal.libsonnet';
local crng = import 'carbon-relay-ng/crng.libsonnet';

k + crng + {
  _images+:: {
  },
  _config+:: {
    namespace: 'mynamespace',
    crng_route_host: 'https://tsdb-1-<instance name>.hosted-metrics.grafana.net/metrics',
    crng_user_id: 'api_key',
    crng_api_key: '<base64 api key>'
  }
}

# generating the resource definitions, using tanka's tk command.
# this should output 4 resources:
#   a secret with the base64 encoded api key
#   a configmap with the carbon-relay-ng.ini and storage-schemas.conf
#   a service which forwards to the carbon-relay-ng pod
#   a deployment creating the carbon-relay-ng pod
$ tk show env

```

# Applying the YAML files
For users who don't use ksonnet yet, we also provide yaml files which can be used as templates to
create the necessary resources to deploy carbon-relay-ng.

The yaml files are in the directory [/examples/k8s](https://github.com/grafana/carbon-relay-ng/tree/document_k8s_deployment/examples/k8s).