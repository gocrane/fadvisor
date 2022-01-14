# Fadvisor: Finance Advisor

[![Go Report Card](https://goreportcard.com/badge/github.com/gocrane/fadvisor)](https://goreportcard.com/report/github.com/gocrane/fadvisor)
[![GoDoc](https://godoc.org/github.com/gocrane/fadvisor?status.svg)](https://godoc.org/github.com/gocrane/fadvisor)
[![License](https://img.shields.io/github/license/gocrane/fadvisor)](https://www.apache.org/licenses/LICENSE-2.0.html)
![GoVersion](https://img.shields.io/github/go-mod/go-version/gocrane/fadvisor)

<img src="docs/images/fadvisor.png">

---

fadvisor(finops advisor) provides capability of `FinOps Observalibility`, it can be integrated with Crane to help users to improve the `cost visualization` and `cost optimization`. Also, it can be integrated with your monitoring system as a metric exporter.


fadvisor has a collection of exporters to collect metrics about cost and other metrics. 

 - `exporters` collects metrics guided by `FinOps`.
   - now a cost-exporter is available, and other exporters can be integrated
 - `apiserver` aggregates collected data and calculates cloud cost based on usage and price  
 
# Concept
fadvisor Cost model provides a way to estimate and breakdown the resource price of each container, pod, or other resources in kubernetes. Then, users can have cost insight by viewing cost grouped by labels or other dimensions.

Please note that Cost model is to **estimate cost**, not to replace the billing, because real billing depends on the billing system.

Cost model is an implementation of the cost allocation and showback & chargeback from the FinOps.
	  
	1. The simplest cost model is to estimate a resource price of all nodes or pods by the same price.
	   For example, when compute costs, you can assume all container's cpu & ram unit price is the same, 2$ Core/Hour, 0.3$ Gib/Hour

	2. Advanced cost model is to estimate a resource price by cost breakdown.
	   This theory is based on that price of each cloud machine instance with different instance type and charge type is different.
	   So the containers price in different node type is different.

# Tutorial
Cost-Exporter is a metrics exporter which collects cloud instance price information by calling Cloud Billing API and exports the price information as metrics. 
Any cloud provider can implement the API and Crane will work for the specific Cloud, Tencent Cloud is supported in current release.

## Deploy all components by one command
```
helm install fadvisor deploy/helm/fadvisor -n crane-system  --set cost-exporter.extraArgs.provider={cloud provider, now support qcloud} --set cost-exporter.extraArgs.secretid={{your cloud secret id}} --set cost-exporter.extraArgs.secretkey={{your cloud secret key}}
```
Except cost-exporter, it will install following components in your system by default.
```
dependencies:
  - name: kube-state-metrics
    condition: fadvisor.kube-state-metrics.enabled,kube-state-metrics.enabled
    repository: file://./charts/kube-state-metrics
  - name: node-exporter
    condition: fadvisor.node-exporter.enabled,node-exporter.enabled
    repository: file://./charts/node-exporter
  - name: prometheus
    condition: fadvisor.prometheus.enabled,prometheus.enabled
    repository: file://./charts/prometheus
  - name: grafana
    condition: fadvisor.grafana.enabled,grafana.enabled
    repository: file://./charts/grafana
```

install on local, it will use default config.

```
helm install fadvisor deploy/helm/fadvisor -n crane-system
```

## Install one by one

To install cost-exporter, you must specify cloud provider and your cloud account credentials as secretid & secretkey.
If you are running Crane in your private cloud, default price will be applied. 

```
helm install cost-exporter deploy/helm/fadvisor/charts/cost-exporter -n crane-system --set extraArgs.provider={cloud provider, now support qcloud} --set extraArgs.secretid={{your cloud secret id}} --set extraArgs.secretkey={{your cloud secret key}}
```

install other components
```
helm install kube-state-metrics deploy/helm/fadvisor/charts/kube-state-metrics -n crane-system
helm install node-exporter deploy/helm/fadvisor/charts/node-exporter -n crane-system
helm install prometheus deploy/helm/fadvisor/charts/prometheus -n crane-system
helm install grafana deploy/helm/fadvisor/charts/grafana -n crane-system
```


## Integrated with existing monitoring components
If you have Prometheus and Grafana installed, you can just deploy the exporter and change related configuration.

You can deploy the cost-exporter to your tke cluster to collect the metric, use prometheus to scrape the metric, and following dashboards can be used;

### 1. deploy cost-exporter
#### install by helm
```
helm install cost-exporter deploy/helm/fadvisor/charts/cost-exporter -n crane-system --set extraArgs.provider={cloud provider, now support qcloud} --set extraArgs.secretid={{your cloud secret id}} --set extraArgs.secretkey={{your cloud secret key}}
```

#### install by kubectl
**NOTE**  you must specify your k8s secret id and secret key in yaml, this is used to access Tencent Cloud Cvm API.
```
kubectl create -f deploy/cost-exporter/ -n crane-system
```
the cost-exporter param has secretId and secretKey, you must provide your cloud provider secret
```
containers:
- name: fadvisor-cost-exporter
  image: docker.io/gocrane/fadvisor-cost-exporter:6927f01
  imagePullPolicy: IfNotPresent
  command:
    - /cost-exporter
    - --v=4
    - --provider=qcloud
    - --secretId=
    - --secretKey=
```

### 2. configure the prometheus scrape config and rules
configure following scrape target to your prometheus. 

```
- job_name: "fadvisor-cost-exporter"
    honor_timestamps: true
    scrape_interval: 5m
    scheme: http
    metrics_path: /metrics
    static_configs:
      - targets: ['cost-exporter.crane-system.svc.cluster.local:8081']
```
**NOTE**, except cost-exporter, your prometheus must have scraped the kubernetes metrics including:
 - `kubelet-cadvisor` metrics.
 - `node-exporter` metrics. need to install node-exporter.
 - `kube-state-metrics` metrics. need to install kube-state-metrics.


configure some recording rules to your prometheus.
```
recording_rules.yml:
  groups:
    - name: costs.rules
      interval: 3600s
      rules:
        - expr: |
            sum(label_replace(irate(container_cpu_usage_seconds_total{container!="POD", container!="",image!=""}[1h]), "node", "$1", "instance",  "(.*)")) by (container, pod, node, namespace) * on (node) group_left() avg(avg_over_time(node_cpu_hourly_cost[1h])) by (node)
          record: namespace:container_cpu_usage_costs_hourly:sum_rate
        - expr: |
            sum(label_replace(avg_over_time(container_memory_working_set_bytes{container!="POD",container!="",image!=""}[1h]), "node", "$1", "instance",  "(.*)")) by (container, pod, node, namespace) / 1024.0 / 1024.0 / 1024.0 * on (node) group_left() avg(avg_over_time(node_ram_hourly_cost[1h])) by (node)
          record: namespace:container_memory_usage_costs_hourly:sum_rate
        - expr: |
            avg(avg_over_time(node_cpu_hourly_cost[1h])) by (node)
          record: node:node_cpu_hourly_cost:avg
        - expr: |
            avg(avg_over_time(node_ram_hourly_cost[1h])) by (node)
          record: node:node_ram_hourly_cost:avg
        - expr: |
            avg(avg_over_time(node_total_hourly_cost[1h])) by (node)
          record: node:node_total_hourly_cost:avg

```

### 3. import following grafana dashboards to your grafana
and there are some available grafana dashboards for you if you has installed grafana already.
```
./deploy/helm/fadvisor/charts/grafana/dashboards/cluster-costs.json
./deploy/helm/fadvisor/charts/grafana/dashboards/costs-dimension.json
./deploy/helm/fadvisor/charts/grafana/dashboards/namespace-costs.json

```

#### estimated cluster costs
<img src="docs/images/cluster-costs.png">

#### namespace costs 
<img src="docs/images/namespace-costs.png">

#### topk container costs
<img src="docs/images/costs-dimension.png">


# Dependency
 - kube-state-metrics
 - node-exporter
 - prometheus
 - grafana

