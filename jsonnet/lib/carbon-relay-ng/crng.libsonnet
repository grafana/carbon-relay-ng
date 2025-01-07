local k = import 'ksonnet-util/kausal.libsonnet';
{
  _images+:: {
    carbon_relay_ng: 'grafana/carbon-relay-ng:main',
  },

  _config+:: {
    namespace: error 'must define namespace',
    crng_api_key: error 'must provide b64 encoded api_key',
    crng_route_host: error 'must provide a crng route host',
    crng_user_id: error 'must provide a crng user id',
    crng_replicas: 1,
    crng_config: importstr 'files/carbon-relay-ng.ini',
    storage_schemas: importstr 'files/storage-schemas.conf',
    // You can set storage_aggregation to null if you don't have an
    // aggregation file and want to use Grafana Cloud's default aggregations
    // instead. (And in that case, don't forget to remove the aggregationFile
    // setting in carbon-relay-ng.ini.)
    storage_aggregation: importstr 'files/storage-aggregation.conf',
  },

  local configMap = k.core.v1.configMap,
  carbon_relay_ng_config_map:
    configMap.new('carbon-relay-ng-config') +
    configMap.mixin.metadata.withNamespace($._config.namespace) +
    configMap.withData(
      {
        'carbon-relay-ng.ini': $._config.crng_config,
        'storage-schemas.conf': $._config.storage_schemas,
        [if $._config.storage_aggregation != null then 'storage-aggregation.conf']: $._config.storage_aggregation,
      }
    ),

  local secret = k.core.v1.secret,
  carbon_relay_ng_secret:
    secret.new(
      'crng-metrics-key',
      {
        api_key: std.base64($._config.crng_api_key),
      },
      'Opaque'
    ) +
    secret.mixin.metadata.withNamespace($._config.namespace),

  local container = k.core.v1.container,
  carbon_relay_ng_container::
    container
    .new('carbon-relay-ng', $._images.carbon_relay_ng) +
    container.withImagePullPolicy('Always') +
    container.withPorts(k.core.v1.containerPort.new('carbon', 2003)) +
    container.withEnv([
      container.envType.fromFieldPath('INSTANCE', 'metadata.name'),
      container.envType.new('GRAFANA_NET_ADDR', $._config.crng_route_host),
      container.envType.new('GRAFANA_NET_USER_ID', $._config.crng_user_id),
      container.envType.fromSecretRef('GRAFANA_NET_API_KEY', 'crng-metrics-key', 'api_key'),
    ]) +
    k.util.resourcesLimits('4', '10Gi') +
    k.util.resourcesRequests('1', '1Gi'),

  local deployment = k.apps.v1.deployment,
  carbon_relay_ng_deployment:
    deployment.new('carbon-relay-ng', $._config.crng_replicas, [$.carbon_relay_ng_container]) +
    deployment.mixin.metadata.withNamespace($._config.namespace) +
    deployment.mixin.metadata.withLabels({ name: 'carbon-relay-ng' }) +
    deployment.mixin.spec.selector.withMatchLabels({ name: 'carbon-relay-ng' }) +
    deployment.mixin.spec.template.spec.withTerminationGracePeriodSeconds(30) +
    deployment.mixin.spec.strategy.rollingUpdate.withMaxSurge(1) +
    deployment.mixin.spec.strategy.rollingUpdate.withMaxUnavailable(0) +
    k.util.configVolumeMount('carbon-relay-ng-config', '/conf'),

  local service = k.core.v1.service,
  carbon_relay_ng_service:
    k.util.serviceFor($.carbon_relay_ng_deployment) +
    service.mixin.metadata.withNamespace($._config.namespace),
}
