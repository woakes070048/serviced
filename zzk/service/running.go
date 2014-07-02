package service

import (
	"strconv"

	"github.com/zenoss/serviced/coordinator/client"
	"github.com/zenoss/serviced/dao"
	"github.com/zenoss/serviced/domain"
	"github.com/zenoss/serviced/domain/service"
	"github.com/zenoss/serviced/domain/servicestate"
)

// NewRunningService instantiates a RunningService object from a given service and service state
func NewRunningService(service *service.Service, state *servicestate.ServiceState) (*dao.RunningService, error) {
	rs := &dao.RunningService{
		ID:              state.ID,
		ServiceID:       state.ServiceID,
		StartedAt:       state.Started,
		HostID:          state.HostID,
		DockerID:        state.DockerID,
		InstanceID:      state.InstanceID,
		Startup:         service.Startup,
		Name:            service.Name,
		Description:     service.Description,
		Instances:       service.Instances,
		PoolID:          service.PoolID,
		ImageID:         service.ImageID,
		DesiredState:    service.DesiredState,
		ParentServiceID: service.ParentServiceID,
	}

	rs.MonitoringProfile.MetricConfigs = make([]domain.MetricConfig, len(service.MonitoringProfile.MetricConfigs))
	build, err := domain.NewMetricConfigBuilder("/metrics/api/performance/query", "POST")
	if err != nil {
		return nil, err
	}
	for i, metricGroup := range service.MonitoringProfile.MetricConfigs {
		for _, metric := range metricGroup.Metrics {
			metricBuilder := build.Metric(metric.ID, metric.Name)
			metricBuilder.SetTag("controlplane_instance_id", strconv.FormatInt(int64(rs.InstanceID), 10))
			metricBuilder.SetTag("controlplane_service_id", rs.ServiceID)
		}
		config, err := build.Config(metricGroup.ID, metricGroup.Name, metricGroup.Description, "1h-ago")
		if err != nil {
			return nil, err
		}
		rs.MonitoringProfile.MetricConfigs[i] = *config
	}
	return rs, nil
}

// LoadRunningService returns a RunningService object given a coordinator connection
func LoadRunningService(conn client.Connection, serviceID, ssID string) (*dao.RunningService, error) {
	var service ServiceNode
	if err := conn.Get(servicepath(serviceID), &service); err != nil {
		return nil, err
	}

	var state ServiceStateNode
	if err := conn.Get(servicepath(serviceID, ssID), &state); err != nil {
		return nil, err
	}

	return NewRunningService(service.Service, state.ServiceState)
}

// LoadRunningServicesByHost returns a slice of RunningServices given a host(s)
func LoadRunningServicesByHost(conn client.Connection, hostIDs ...string) ([]*dao.RunningService, error) {
	var rss []*dao.RunningService
	for _, hostID := range hostIDs {
		stateIDs, err := conn.Children(hostpath(hostID))
		if err != nil {
			return nil, err
		}
		for _, ssID := range stateIDs {
			var hs HostState
			if err := conn.Get(hostpath(hostID, ssID), &hs); err != nil {
				return nil, err
			}

			rs, err := LoadRunningService(conn, hs.ServiceID, hs.ServiceStateID)
			if err != nil {
				return nil, err
			}

			rss = append(rss, rs)
		}
	}
	return rss, nil
}

// LoadRunningServicesByService returns a slice of RunningServices per service id(s)
func LoadRunningServicesByService(conn client.Connection, serviceIDs ...string) ([]*dao.RunningService, error) {
	var rss []*dao.RunningService
	for _, serviceID := range serviceIDs {
		stateIDs, err := conn.Children(servicepath(serviceID))
		if err != nil {
			return nil, err
		}
		for _, ssID := range stateIDs {
			rs, err := LoadRunningService(conn, serviceID, ssID)
			if err != nil {
				return nil, err
			}
			rss = append(rss, rs)
		}
	}
	return rss, nil
}

// LoadRunningServices gets all RunningServices
func LoadRunningServices(conn client.Connection) ([]*dao.RunningService, error) {
	serviceIDs, err := conn.Children(servicepath())
	if err != nil {
		return nil, err
	}

	// filter non-unique service ids
	unique := make(map[string]interface{})
	ids := make([]string, 0)
	for _, serviceID := range serviceIDs {
		if _, ok := unique[serviceID]; !ok {
			unique[serviceID] = nil
			ids = append(ids, serviceID)
		}
	}

	return LoadRunningServicesByService(conn, ids...)
}
