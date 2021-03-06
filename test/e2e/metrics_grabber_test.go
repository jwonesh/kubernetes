/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"strings"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/metrics"
	"k8s.io/kubernetes/pkg/util/sets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Missing = Assumed minus Observed, Invalid = Observed minus Assumed
func validateLabelSet(labelSet map[string][]string, data metrics.Metrics, invalidLabels map[string]sets.String, missingLabels map[string]sets.String) {
	for metric, labels := range labelSet {
		vector, found := data[metric]
		Expect(found).To(Equal(true))
		if found && len(vector) > 0 {
			for _, observation := range vector {
				for label := range observation.Metric {
					// We need to check if it's a known label for this metric.
					// Omit Prometheus internal metrics.
					if strings.HasPrefix(string(label), "__") {
						continue
					}
					invalidLabel := true
					for _, knownLabel := range labels {
						if string(label) == knownLabel {
							invalidLabel = false
						}
					}
					if invalidLabel {
						if _, ok := invalidLabels[metric]; !ok {
							invalidLabels[metric] = sets.NewString()
						}
						invalidLabels[metric].Insert(string(label))
					}
				}
			}
		}
	}
}

func checkMetrics(response metrics.Metrics, assumedMetrics map[string][]string) {
	invalidLabels := make(map[string]sets.String)
	unknownLabels := make(map[string]sets.String)
	validateLabelSet(metrics.CommonMetrics, response, invalidLabels, unknownLabels)
	validateLabelSet(assumedMetrics, response, invalidLabels, unknownLabels)

	Expect(unknownLabels).To(BeEmpty())
	Expect(invalidLabels).To(BeEmpty())
}

var _ = Describe("MetricsGrabber", func() {
	framework := NewFramework("metrics-grabber")
	var c *client.Client
	var grabber *metrics.MetricsGrabber
	BeforeEach(func() {
		var err error
		c = framework.Client
		expectNoError(err)
		grabber, err = metrics.NewMetricsGrabber(c, true, true, true, true)
		expectNoError(err)
	})

	It("should grab all metrics from API server.", func() {
		By("Connecting to /metrics endpoint")
		unknownMetrics := sets.NewString()
		response, err := grabber.GrabFromApiServer(unknownMetrics)
		expectNoError(err)
		Expect(unknownMetrics).To(BeEmpty())

		checkMetrics(metrics.Metrics(response), metrics.KnownApiServerMetrics)
	})

	It("should grab all metrics from a Kubelet.", func() {
		// We run this test only on GCE, as for some reason it flakes in GKE #19468
		if providerIs("gce") {
			By("Connecting proxying to Node through the API server")
			nodes := ListSchedulableNodesOrDie(c)
			Expect(nodes.Items).NotTo(BeEmpty())
			unknownMetrics := sets.NewString()
			response, err := grabber.GrabFromKubelet(nodes.Items[0].Name, unknownMetrics)
			expectNoError(err)
			Expect(unknownMetrics).To(BeEmpty())

			checkMetrics(metrics.Metrics(response), metrics.KnownKubeletMetrics)
		}
	})

	It("should grab all metrics from a Scheduler.", func() {
		By("Connecting proxying to Pod through the API server")
		// Check if master Node is registered
		nodes, err := c.Nodes().List(api.ListOptions{})
		expectNoError(err)

		var masterRegistered = false
		for _, node := range nodes.Items {
			if strings.HasSuffix(node.Name, "master") {
				masterRegistered = true
			}
		}
		if !masterRegistered {
			Logf("Master is node registered. Skipping testing Scheduler metrics.")
			return
		}
		unknownMetrics := sets.NewString()
		response, err := grabber.GrabFromScheduler(unknownMetrics)
		expectNoError(err)
		Expect(unknownMetrics).To(BeEmpty())

		checkMetrics(metrics.Metrics(response), metrics.KnownSchedulerMetrics)
	})

	It("should grab all metrics from a ControllerManager.", func() {
		By("Connecting proxying to Pod through the API server")
		// Check if master Node is registered
		nodes, err := c.Nodes().List(api.ListOptions{})
		expectNoError(err)

		var masterRegistered = false
		for _, node := range nodes.Items {
			if strings.HasSuffix(node.Name, "master") {
				masterRegistered = true
			}
		}
		if !masterRegistered {
			Logf("Master is node registered. Skipping testing ControllerManager metrics.")
			return
		}
		unknownMetrics := sets.NewString()
		response, err := grabber.GrabFromControllerManager(unknownMetrics)
		expectNoError(err)
		Expect(unknownMetrics).To(BeEmpty())

		checkMetrics(metrics.Metrics(response), metrics.KnownControllerManagerMetrics)
	})
})
