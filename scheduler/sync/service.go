package sync

import (
	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/dao"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/service"
	"github.com/control-center/serviced/facade"
	"github.com/control-center/serviced/sync"
	zkservice "github.com/control-center/serviced/zzk/service"
)

func ConvertServices(svcs []service.Service) []sync.Datum {
	data := make([]sync.Datum, len(svcs))
	for i, svc := range svcs {
		data[i] = zkservice.ServiceNode{Service: &svc}
	}
	return data
}

type FacadeServiceSource struct {
	facade *facade.Facade
	poolID string
}

func (source *FacadeServiceSource) Init(facade *facade.Facade, poolID string) sync.Source {
	return &FacadeServiceSource{facade, poolID}
}

func (source *FacadeServiceSource) Get() ([]sync.Datum, error) {
	svcs, err := source.facade.GetServices(datastore.Get(), dao.ServiceRequest{})
	if err != nil {
		return nil, err
	}
	return ConvertServices(svcs), nil
}

func (source *FacadeServiceSource) Add(datum sync.Datum) error {
	var svc service.Service
	if sd, ok := datum.(zkservice.ServiceNode); ok {
		svc = *sd.Service
	} else {
		return sync.ErrBadType
	}
	return source.facade.AddService(datastore.Get(), svc)
}

func (source *FacadeServiceSource) Update(datum sync.Datum) error {
	var svc service.Service
	if sd, ok := datum.(zkservice.ServiceNode); ok {
		svc = *sd.Service
	} else {
		return sync.ErrBadType
	}
	return source.facade.UpdateService(datastore.Get(), svc)
}

func (source *FacadeServiceSource) Delete(id string) error {
	// we only want to delete the service if it is in the same pool
	svc, err := source.facade.GetService(datastore.Get(), id)
	if err != nil {
		return err
	} else if svc.PoolID != source.poolID {
		return nil
	}
	return source.facade.RemoveService(datastore.Get(), id)
}

type ZKServiceSource struct {
	conn client.Connection
}

func (source *ZKServiceSource) Init(conn client.Connection) sync.Source {
	return &ZKServiceSource{conn}
}

func (source *ZKServiceSource) Get() ([]sync.Datum, error) {
	svcs, err := zkservice.GetServices(source.conn)
	if err != nil {
		return nil, err
	}
	return ConvertServices(svcs), nil
}

func (source *ZKServiceSource) Add(datum sync.Datum) error {
	var svc service.Service
	if sd, ok := datum.(zkservice.ServiceNode); ok {
		svc = *sd.Service
	} else {
		return sync.ErrBadType
	}
	return zkservice.UpdateService(source.conn, &svc)
}

func (source *ZKServiceSource) Update(datum sync.Datum) error {
	return source.Add(datum)
}

func (source *ZKServiceSource) Delete(id string) error {
	mutex := zkservice.ServiceLock(source.conn)
	mutex.Lock()
	defer mutex.Unlock()
	return zkservice.RemoveService(source.conn, id)
}
