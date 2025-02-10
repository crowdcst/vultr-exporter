package collector

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vultr/govultr/v2"
)

// LoadBalancerCollector represents Load Balancers
type LoadBalancerCollector struct {
	System    System
	Client    *govultr.Client
	Log       logr.Logger
	Up        *prometheus.Desc
	Instances *prometheus.Desc
}

// NewLoadBalancerCollector creates a new LoadBalancerCollector
func NewLoadBalancerCollector(s System, client *govultr.Client, log logr.Logger) *LoadBalancerCollector {
	subsystem := "load_balancer"
	return &LoadBalancerCollector{
		System: s,
		Client: client,
		Log:    log,
		Up: prometheus.NewDesc(
			prometheus.BuildFQName(s.Namespace, subsystem, "up"),
			"Load balancer",
			[]string{
				"label",
				"region",
				"status",
			},
			nil,
		),
		Instances: prometheus.NewDesc(
			prometheus.BuildFQName(s.Namespace, subsystem, "instances"),
			"Number of Load balancer instances",
			[]string{
				"label",
				"region",
				"status",
			},
			nil,
		),
	}
}

// Collect implements Prometheus' Collector interface and is used to collect metrics
func (c *LoadBalancerCollector) Collect(ch chan<- prometheus.Metric) {
	log := c.Log.WithName("Collect")
	ctx := context.Background()
	options := &govultr.ListOptions{}
	loadbalancers, meta, err := c.Client.LoadBalancer.List(ctx, options)
	if err != nil {
		log.Info("Unable to list LoadBalancers")
		return
	}

	log.Info("Response",
		"meta", meta,
	)

	// Create a buffered channel for metrics to avoid blocking
	metricsChan := make(chan prometheus.Metric, len(loadbalancers)*2)

	var wg sync.WaitGroup
	for _, loadbalancer := range loadbalancers {
		wg.Add(1)
		go func(lb govultr.LoadBalancer) {
			defer wg.Done()
			log.Info("Details")

			metricsChan <- prometheus.MustNewConstMetric(
				c.Up,
				prometheus.CounterValue,
				func(status string) (result float64) {
					if status == "active" {
						result = 1.0
					}
					return result
				}(lb.Status),
				[]string{
					lb.Label,
					lb.Region,
					lb.Status,
				}...,
			)
			metricsChan <- prometheus.MustNewConstMetric(
				c.Instances,
				prometheus.CounterValue,
				float64(len(lb.Instances)),
				[]string{
					lb.Label,
					lb.Region,
					lb.Status,
				}...,
			)
		}(loadbalancer)
	}

	// Close metrics channel when all goroutines are done
	go func() {
		wg.Wait()
		close(metricsChan)
	}()

	// Send all metrics to the Prometheus channel
	for metric := range metricsChan {
		ch <- metric
	}
}

// Describe implements Prometheus' Collector interface and is used to Describe metrics
func (c *LoadBalancerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Up
	ch <- c.Instances
}
