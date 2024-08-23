package utils

import (
	"bytes"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"

	"capi-bootstrap/types"
)

func TestTabWriteClusters(t *testing.T) {
	output := `Name	Status	Version		External IP	Age
node1	Ready	v1.29.4+k3s1	192.168.1.1	5.1h
node2	Ready	v1.29.4+k3s1	192.168.1.2	5.0h
node3	Ready	v1.29.4+k3s1	192.168.1.3	5.0h
node4	Ready	v1.29.4+k3s1	192.168.1.4	5.0h
node5	Ready	v1.29.4+k3s1	192.168.1.5	5.0h
node6	Ready	v1.29.4+k3s1	192.168.1.6	5.0h
`

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 8, 1, '\t', 0)
	clusters := []types.ClusterInfo{
		{
			Name: "test-cluster",
			Nodes: []*types.NodeInfo{
				{
					Name:              "node1",
					Status:            "Ready",
					Version:           "v1.29.4+k3s1",
					ExternalIP:        "192.168.1.1",
					DaysSinceCreation: "5.1h",
				},
				{
					Name:              "node2",
					Status:            "Ready",
					Version:           "v1.29.4+k3s1",
					ExternalIP:        "192.168.1.2",
					DaysSinceCreation: "5.0h",
				},
				{
					Name:              "node3",
					Status:            "Ready",
					Version:           "v1.29.4+k3s1",
					ExternalIP:        "192.168.1.3",
					DaysSinceCreation: "5.0h",
				},
				{
					Name:              "node4",
					Status:            "Ready",
					Version:           "v1.29.4+k3s1",
					ExternalIP:        "192.168.1.4",
					DaysSinceCreation: "5.0h",
				},
				{
					Name:              "node5",
					Status:            "Ready",
					Version:           "v1.29.4+k3s1",
					ExternalIP:        "192.168.1.5",
					DaysSinceCreation: "5.0h",
				},
				{
					Name:              "node6",
					Status:            "Ready",
					Version:           "v1.29.4+k3s1",
					ExternalIP:        "192.168.1.6",
					DaysSinceCreation: "5.0h",
				},
			},
		},
	}
	assert.NoError(t, TabWriteClusters(w, clusters))
	assert.NoError(t, w.Flush())
	assert.Equal(t, output, buf.String())
}
