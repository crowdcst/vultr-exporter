package collector

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/vultr/govultr/v2"
)

// KubernetesCollector represents Kubernetes Engine
type KubernetesCollector struct {
	System    System
	Client    *govultr.Client
	Log       logr.Logger
	Up        *prometheus.Desc
	NodePools *prometheus.Desc
	Nodes     *prometheus.Desc
}

// NewKubernetesCollector creates a new KubernetesCollector
func NewKubernetesCollector(s System, client *govultr.Client, log logr.Logger) *KubernetesCollector {
	subsystem := "kubernetes"
	return &KubernetesCollector{
		System: s,
		Client: client,
		Log:    log,
		Up: prometheus.NewDesc(
			prometheus.BuildFQName(s.Namespace, subsystem, "cluster_up"),
			"Kubernetes cluster",
			[]string{
				"cluster_id",
				"label",
				"region",
				"version",
				"status",
			},
			nil,
		),
		NodePools: prometheus.NewDesc(
			prometheus.BuildFQName(s.Namespace, subsystem, "node_pool"),
			"Number of Node Pools",
			[]string{
				"cluster_id",
				"label",
				"region",
				"version",
				"status",
			},
			nil,
		),
		Nodes: prometheus.NewDesc(
			prometheus.BuildFQName(s.Namespace, subsystem, "node"),
			"Number of Nodes",
			[]string{
				"cluster_id",
				"label",
				"plan",
				"status",
				"tag",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *KubernetesCollector) Collect(ch chan<- prometheus.Metric) {
	log := c.Log.WithName("Collect")
	ctx := context.Background()
	options := &govultr.ListOptions{}
	clusters, meta, err := c.Client.Kubernetes.ListClusters(ctx, options)
	if err != nil {
		log.Info("Unable to ListClusters")
		return
	}

	log.Info("Response",
		"meta", meta,
	)

	// Enumerate all of the clusters
	var wg sync.WaitGroup
	for _, cluster := range clusters {
		wg.Add(1)
		go func(cluster govultr.Cluster) {
			defer wg.Done()
			log.Info("Details",
				"Cluster", cluster,
			)

			ch <- prometheus.MustNewConstMetric(
				c.Up,
				prometheus.CounterValue,
				func(status string) (result float64) {
					if status == "active" {
						result = 1.0
					}
					return result
				}(cluster.Status),
				[]string{
					cluster.ID,
					cluster.Label,
					cluster.Region,
					cluster.Version,
					cluster.Status,
				}...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.NodePools,
				prometheus.GaugeValue,
				float64(len(cluster.NodePools)),
				[]string{
					cluster.ID,
					cluster.Label,
					cluster.Region,
					cluster.Version,
					cluster.Status,
				}...,
			)
			for _, nodepool := range cluster.NodePools {
				log.Info("Debugging",
					"NodeQuantity", nodepool.NodeQuantity,
					"len(Nodes)", len(nodepool.Nodes),
				)
				ch <- prometheus.MustNewConstMetric(
					c.Nodes,
					prometheus.GaugeValue,
					float64(nodepool.NodeQuantity),
					[]string{
						cluster.ID,
						nodepool.Label,
						nodepool.Plan,
						nodepool.Status,
						nodepool.Tag,
					}...,
				)
			}
		}(cluster)
	}
	wg.Wait()

}

// Describe implements Prometheus' Collector interface and is used to describe metrics
func (c *KubernetesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Up
	ch <- c.NodePools
	ch <- c.Nodes
}
