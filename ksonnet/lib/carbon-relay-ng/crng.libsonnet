{
  _images+:: {
    carbon_relay_ng: 'raintank/carbon-relay-ng:master'
  },

  _config+:: {
    namespace: error 'must define namespace',
    crng_api_key: error 'must provide b64 encoded api_key',
    crng_route_host: error 'must provide a crng route host',
    crng_user_id: error 'must provide a crng user id',
    crng_replicas: 1,
    crng_config: importstr 'files/carbon-relay-ng.ini',
    storage_schemas: importstr 'files/storage-schemas.conf',
  },

  local configMap = $.core.v1.configMap,
  carbon_relay_ng_config_map:
    configMap.new('carbon-relay-ng-config') +
    configMap.mixin.metadata.withNamespace($._config.namespace) +
    configMap.withData(
      {
        'carbon-relay-ng.ini': $._config.crng_config,
        'storage-schemas.conf': $._config.storage_schemas,
      }
    ),
  
  local secret = $.core.v1.secret,
  carbon_relay_ng_secret:
    secret.new(
      'crng-metrics-key',
      {
        api_key: std.base64($._config.crng_api_key),
      },
      'Opaque'
    ) +
    secret.mixin.metadata.withNamespace($._config.namespace),

  local container = $.core.v1.container,
  carbon_relay_ng_container::
    container
    .new('carbon-relay-ng', $._images.carbon_relay_ng)
    .withImagePullPolicy('Always') +
    container.withPorts($.core.v1.containerPort.new('carbon', 2003)) +
    container.withEnv([
      container.envType.fromFieldPath('INSTANCE', 'metadata.name'),
      container.envType.new('GRAFANA_NET_ADDR', $._config.crng_route_host),
      container.envType.new('GRAFANA_NET_USER_ID', $._config.crng_user_id),
      container.envType.fromSecretRef('GRAFANA_NET_API_KEY', 'crng-metrics-key', 'api_key'),
    ]) +
    $.util.resourcesLimits('4', '10Gi') +
    $.util.resourcesRequests('1', '1Gi'),

  local deployment = $.apps.v1beta1.deployment,
  carbon_relay_ng_deployment:
    deployment.new('carbon-relay-ng', $._config.crng_replicas, [$.carbon_relay_ng_container]) +
    deployment.mixin.metadata.withNamespace($._config.namespace) +
    deployment.mixin.metadata.withLabels({ name: 'carbon-relay-ng' }) +
    deployment.mixin.spec.selector.withMatchLabels({ name: 'carbon-relay-ng' }) +
    deployment.mixin.spec.template.spec.withTerminationGracePeriodSeconds(30) +
    deployment.mixin.spec.strategy.rollingUpdate.withMaxSurge(1) +
    deployment.mixin.spec.strategy.rollingUpdate.withMaxUnavailable(0) +
    $.util.configVolumeMount('carbon-relay-ng-config', '/conf'),

  local service = $.core.v1.service,
  carbon_relay_ng_service:
    $.util.serviceFor($.carbon_relay_ng_deployment) +
    service.mixin.metadata.withNamespace($._config.namespace)
}
