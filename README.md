# An (primitive) Envoy ALS Implementation

Istio routing and debug can be _hard_.
Probably not for istio vets, but nearly a year into consistently using Istio, even I get tripped up with certain situations and need some good ole access logs to see just what's going on.
This ranges from `protocolDetectionTimeout` issues (due to lovely connection pools opening connections they don't use for whole minutes or more) to broken/mismatched routes/serviceEntries/destinationRules.
And while a great many of these issues are certainly the byproduct of my own silliness or poor automation/config management, I'm often working with people whose workloads I'm unfamiliar with.
And especially when that's the case it's easy to get caught in a situation where I need more help!

So as I debug and explore the istio/envoy configs, after I've exhausted my `istioctl`-foo and turned over every rock I can think of, I turn to access logs.
(Fortunately, this isn't nearly as often as it used to be).
But Istio (as of at least 1.6) only has global access logging as a first-class config!
Turn it on and it's on for everything...which is less than ideal in a lot of situations.

So not satisfied with the situations, I started checking out how Envoy might best be persuaded to do as I need.
I stumbled on the new(er) field on the [Istio annotations](https://istio.io/latest/docs/reference/config/annotations/) page, `proxy.istio.io/config`.
Sweet; now I can modify _individual_ pods instead of the whole mesh.
I've been waiting for exactly this (and excuses to use it)! (see the doc :))

I found this on the heels of another gem: [Access Loggers](https://www.envoyproxy.io/docs/envoy/latest/api-v2/config/filter/accesslog/v2/accesslog.proto).
I knew these must be in there somewhere, but this was my first dive into Envoy directly: I'd relied almost solely on Istio's configs to do as I needed.
Well, great: it's a gRPC service with well-maintained and documented proto files; beautiful start.
So I pursued the use of these together to get some more granular control over deployments I work with.

One thing led to another and after lots of doc scanning (Envoy config was a little hard to navigate at first), the very simple impl in this repo is where I landed.
It simply exposes both available gRPC API versions (v2/3) in a simple go program.
Using k8s to deploy the service and some simple istio config, and it was time to begin the hotwiring!

Technically, this Envoy ALS impl isn't technically required for granular logging.
Why?
Because we can also configure the file-based logging in the same way.
It was only after I did all this I stumbled on the alternative (oops).
Still a valuable thing to have done, though.
Hope it helps whomever find this!

That's all to the story; the rest is how it works and the results should be pretty obvious ;)
Check out the build & deploy step and the following istio config to get yourself wired up and ready to go!

And don't forget: if all you need is file-based access logging, skip _all_ the deployment and istio config and focus only on that last `EnvoyFilter` (see the inline comments).

## Build & Deploy

```bash
docker build -t envoy-als-impl -f Dockerfile .
# if not using minikube:
# docker push ...
```

K8s:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: envoy-als
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: envoy-als
  template:
    metadata:
      annotations:
        sidecar.istio.io/inject: "false"
      labels:
        app: envoy-als
    spec:
      containers:
      - name: envoy-als
        # fine for minikube but obviously needs to be pushed somewhere
        image: envoy-als-impl
        args: ["-port", "14000"]
        ports:
        - containerPort: 14000
---
apiVersion: v1
kind: Service
metadata:
  name: envoy-als
  namespace: istio-system
spec:
  selector:
    app: envoy-als
  type: ClusterIP
  ports:
  - name: grpc
    port: 14000
    protocol: TCP
    targetPort: 14000
```

## How to use with Istio

This works with a largely standard istio deployment; extra EnvoyFilters or custom configs, e.g., may impact how this functions.

This istio [proxy config](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig) adds a cluster to the envoy `static_resources` field named `envoy_accesslog_service`.
It's used in the `EnvoyFilter` grpc config for the access log grpc sink.
```yaml
# either in the base istio config map
# OR with the annotation `proxy.istio.io/config`
envoyAccessLogService:
  address: "envoy-als.istio-system:14000"
  tlsSettings:
    # ISTIO_MUTUAL caused problems for unknown reasons...should probably be done with _some_ wire security
    mode: "DISABLE"
```

[EnvoyFilter](https://istio.io/latest/docs/reference/config/networking/envoy-filter/#EnvoyFilter) used to hotwire the sidecar proxy.
Add during or before runtime: Envoy picks up the changes regardless and dynamically patches its config.
If this is a temporary access logging situation, simply delete this resource when you're done and Envoy reverts back to doing as it did before.
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: als-tweaks
  # setting the namespace to istio-system--or wherever istio's config is--can (somewhat) emulate the global access logging for all sidecars
  # see the doc linked above
  # namespace: ???
spec:
  # don't forget to modify this as appropriate!
  # if it's missing, certain semantics apply: check the doc link above
  #workloadSelector:
  #  labels:
  #    app: yourapphere
  configPatches:
  - applyTo: NETWORK_FILTER # http connection manager is a filter in Envoy
    match:
      context: SIDECAR_INBOUND
      listener:
        filterChain:
          filter:
            name: "envoy.http_connection_manager"
    patch:
      operation: MERGE
      value:
        name: "envoy.http_connection_manager"
        typed_config:
          # this could easily change across istio versions (e.g. v2 -> v3), however it's required
          # so do be careful across upgrades!
          "@type": "type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager"
          access_log:
          # optionally use file logging (fine for GKE where logs are already aggregated)
          - name: envoy.access_loggers.file
            config:
              path: /dev/stdout
          # for when local logging isn't sufficient
          - name: envoy.access_loggers.http_grpc
            config:
              common_config:
                # obviously rename this
                log_name: foobar
                grpc_service:
                  envoy_grpc:
                    # see previous istio proxy config block for where this name comes from
                    cluster_name: envoy_accesslog_service
```
