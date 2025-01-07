# Deploying Carbon-Relay-Ng on Kubernetes

We support two methods of deploying Carbon-Relay-Ng onto Kubernetes, via Tanka and by directly applying the provided
`.yaml` files.

# Tanka

In this repository in `/jsonnet/lib/carbon-relay-ng` we provide a jsonnet library which uses the kubernetes library from
the [ksonnet project](https://github.com/ksonnet/ksonnet-lib) to create a simple carbon-relay-ng deployment. This
library also depends on the [ksonnet-util library](https://github.com/grafana/jsonnet-libs/tree/master/ksonnet-util) for
some helper functions.

Using [Tanka](https://tanka.dev/) and [jsonnet-bundler](https://github.com/jsonnet-bundler/jsonnet-bundler), a simple
carbon-relay-ng deployment can be created like shown here:

Initialize a Tanka environment, this will setup a directory structure and pull in the kubernetes and ksonnet-util
libraries.

```
tk init
```

Install the carbon-relay-ng jsonnet lib:

```
jb install 'github.com/grafana/carbon-relay-ng/jsonnet/lib/carbon-relay-ng'
```

Now we can setup carbon-relay-ng in an environment:

```
# environments/default/main.jsonnet
local crng = import 'carbon-relay-ng/crng.libsonnet';

{
  crng: crng {
    _config+:: {
      namespace: 'mynamespace',
      crng_route_host: '<graphite endpoint>/metrics',
      crng_user_id: '<user id>',
      crng_api_key: '<base64 api key>',
    },
    _images+:: {
    },
  },
}
```

Tanka can generate the resource definitions, the output should yield 4 resources:
* a secret with the base64 encoded api key
* a configmap with the carbon-relay-ng.ini and storage-schemas.conf, and, optionally, storage-aggregation.conf
* a service which forwards to the carbon-relay-ng pod
* a deployment creating the carbon-relay-ng pod

```
tk show environments/default
```

Before we apply this, we need to setup the server:

```
tk env set environments/default --server <k8s endpoint>
# or
tk env set environments/default --server-from-context
```

Then apply it:

```
tk apply environments/default
```

# Applying the YAML files
For users who don't use Tanka, we also provide yaml files which can be used as templates to
create the necessary resources to deploy carbon-relay-ng.

The yaml files are in the directory [/examples/k8s](https://github.com/grafana/carbon-relay-ng/tree/main/examples/k8s).
