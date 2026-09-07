package remote

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendHostPressure(t *testing.T) {
	var body map[string]interface{}
	c, server := createTestClient(func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/nodes/pressure", r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		assert.NoError(t, json.Unmarshal(b, &body))
		rw.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	err := c.SendHostPressure(context.Background(), HostPressureRequest{
		Previous: "ok",
		Current:  "critical",
		Snapshot: map[string]interface{}{"cpu": map[string]interface{}{"percent": 97.5}},
	})

	assert.NoError(t, err)
	assert.Equal(t, "ok", body["previous"])
	assert.Equal(t, "critical", body["current"])
	assert.Equal(t, 97.5, body["snapshot"].(map[string]interface{})["cpu"].(map[string]interface{})["percent"])
}

func TestSendHostPressureReturnsRequestErrors(t *testing.T) {
	c, server := createTestClient(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		_, _ = rw.Write([]byte(`{"errors":[{"code":"NotFoundHttpException","status":"404","detail":"Not found"}]}`))
	})
	defer server.Close()

	err := c.SendHostPressure(context.Background(), HostPressureRequest{Previous: "ok", Current: "warning"})

	rerr := AsRequestError(err)
	if assert.NotNil(t, rerr) {
		assert.Equal(t, http.StatusNotFound, rerr.StatusCode())
	}
}
