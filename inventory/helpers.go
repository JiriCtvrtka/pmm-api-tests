package inventory

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/google/uuid"
	"github.com/percona/pmm/api/inventory/json/client"
	"github.com/percona/pmm/api/inventory/json/client/agents"
	"github.com/percona/pmm/api/inventory/json/client/nodes"
	"github.com/percona/pmm/api/inventory/json/client/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/Percona-Lab/pmm-api-tests" // init default client
)

func withUUID(t *testing.T, name string) string {
	t.Helper()
	hostname, err := os.Hostname()
	require.NoError(t, err)
	random, err := uuid.NewRandom()
	require.NoError(t, err)

	return fmt.Sprintf("test-for-%s-%s-%s", hostname, name, random.String())
}

func removeNodes(t *testing.T, nodeIDs ...string) {
	t.Helper()
	for _, nodeID := range nodeIDs {
		params := &nodes.RemoveNodeParams{
			Body:    nodes.RemoveNodeBody{NodeID: nodeID},
			Context: context.TODO(),
		}
		res, err := client.Default.Nodes.RemoveNode(params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	}
}

func addGenericNode(t *testing.T, nodeName string) *nodes.AddGenericNodeOKBody {
	t.Helper()
	params := &nodes.AddGenericNodeParams{
		Body: nodes.AddGenericNodeBody{
			NodeName: nodeName,
		},
		Context: context.TODO(),
	}
	res, err := client.Default.Nodes.AddGenericNode(params)
	assert.NoError(t, err)
	require.NotNil(t, res)
	return res.Payload
}

func addRemoteNode(t *testing.T, nodeName string) *nodes.AddRemoteNodeOKBody {
	t.Helper()
	params := &nodes.AddRemoteNodeParams{
		Body: nodes.AddRemoteNodeBody{
			NodeName: nodeName,
		},
		Context: context.TODO(),
	}
	res, err := client.Default.Nodes.AddRemoteNode(params)
	assert.NoError(t, err)
	require.NotNil(t, res)
	return res.Payload
}

func removeServices(t *testing.T, serviceIDs ...string) {
	t.Helper()
	for _, serviceID := range serviceIDs {
		params := &services.RemoveServiceParams{
			Body:    services.RemoveServiceBody{ServiceID: serviceID},
			Context: context.TODO(),
		}
		res, err := client.Default.Services.RemoveService(params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	}
}

func addMySQLService(t *testing.T, body services.AddMySQLServiceBody) *services.AddMySQLServiceOKBody {
	t.Helper()
	params := &services.AddMySQLServiceParams{
		Body:    body,
		Context: context.TODO(),
	}
	res, err := client.Default.Services.AddMySQLService(params)
	assert.NoError(t, err)
	require.NotNil(t, res)
	return res.Payload
}

func removeAgents(t *testing.T, agentIDs ...string) {
	t.Helper()
	for _, agentID := range agentIDs {
		params := &agents.RemoveAgentParams{
			Body:    agents.RemoveAgentBody{AgentID: agentID},
			Context: context.TODO(),
		}
		res, err := client.Default.Agents.RemoveAgent(params)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	}
}

func addPMMAgent(t *testing.T, node string) *agents.AddPMMAgentOKBody {
	t.Helper()
	res, err := client.Default.Agents.AddPMMAgent(&agents.AddPMMAgentParams{
		Body:    agents.AddPMMAgentBody{NodeID: node},
		Context: context.TODO(),
	})
	assert.NoError(t, err)
	require.NotNil(t, res)
	return res.Payload
}

func addMySqldExporter(t *testing.T, body agents.AddMySqldExporterBody) *agents.AddMySqldExporterOKBody {
	t.Helper()
	res, err := client.Default.Agents.AddMySqldExporter(&agents.AddMySqldExporterParams{
		Body:    body,
		Context: context.TODO(),
	})
	assert.NoError(t, err)
	require.NotNil(t, res)
	return res.Payload
}

func assertEqualAPIError(t *testing.T, err error, expectedCode int) bool {
	t.Helper()
	expectedError := &runtime.APIError{
		OperationName: "unknown error",
		Code:          expectedCode,
	}
	assert.Error(t, err)
	err.(*runtime.APIError).Response = nil
	return assert.Equal(t, expectedError, err)
}

func assertMySQLServiceExists(t *testing.T, res *services.ListServicesOK, serviceID string) bool {
	t.Helper()
	return assert.Conditionf(t, func() (success bool) {
		for _, v := range res.Payload.Mysql {
			if v.ServiceID == serviceID {
				return true
			}
		}
		return false
	}, "There should be MySQL service with id `%s`", serviceID)
}

func assertMySQLServiceNotExist(t *testing.T, res *services.ListServicesOK, serviceID string) bool {
	t.Helper()
	return assert.Conditionf(t, func() (success bool) {
		for _, v := range res.Payload.Mysql {
			if v.ServiceID == serviceID {
				return false
			}
		}
		return true
	}, "There should not be MySQL service with id `%s`", serviceID)
}

func assertMySQLExporterExists(t *testing.T, res *agents.ListAgentsOK, mySqldExporterID string) bool {
	return assert.Conditionf(t, func() (success bool) {
		for _, v := range res.Payload.MysqldExporter {
			if v.AgentID == mySqldExporterID {
				return true
			}
		}
		return false
	}, "There should be MySQL agent with id `%s`", mySqldExporterID)
}

func assertMySQLExporterNotExists(t *testing.T, res *agents.ListAgentsOK, mySqldExporterID string) bool {
	return assert.Conditionf(t, func() (success bool) {
		for _, v := range res.Payload.MysqldExporter {
			if v.AgentID == mySqldExporterID {
				return false
			}
		}
		return true
	}, "There should be MySQL agent with id `%s`", mySqldExporterID)
}

func assertPMMAgentExists(t *testing.T, res *agents.ListAgentsOK, pmmAgentID string) bool {
	return assert.Conditionf(t, func() (success bool) {
		for _, v := range res.Payload.PMMAgent {
			if v.AgentID == pmmAgentID {
				return true
			}
		}
		return false
	}, "There should be PMM-agent with id `%s`", pmmAgentID)
}

func assertPMMAgentNotExists(t *testing.T, res *agents.ListAgentsOK, pmmAgentID string) bool {
	return assert.Conditionf(t, func() (success bool) {
		for _, v := range res.Payload.PMMAgent {
			if v.AgentID == pmmAgentID {
				return false
			}
		}
		return true
	}, "There should be PMM-agent with id `%s`", pmmAgentID)
}