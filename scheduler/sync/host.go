package sync

import (
	"time"

	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/host"
	"github.com/control-center/serviced/facade"
	"github.com/control-center/serviced/sync"
	zkhost "github.com/control-center/serviced/zzk/service"
)

func ConvertHosts(hosts []host.Host) []sync.Datum {
	data := make([]sync.Datum, len(hosts))
	for i := range hosts {
		data[i] = zkhost.HostNode{Host: &hosts[i]}
	}
	return data
}

type FacadeHostSource struct {
	facade *facade.Facade
	poolID string
}

func (source *FacadeHostSource) Init(facade *facade.Facade, poolID string) sync.Source {
	return &FacadeHostSource{facade, poolID}
}

func (source *FacadeHostSource) Get() ([]sync.Datum, error) {
	hosts, err := source.facade.GetHosts(datastore.Get())
	if err != nil {
		return nil, err
	}

	return ConvertHosts(hosts), nil
}

func (source *FacadeHostSource) Add(datum sync.Datum) error {
	var host host.Host
	if hd, ok := datum.(zkhost.HostNode); ok {
		host = *hd.Host
	} else {
		return sync.ErrBadType
	}

	return source.facade.AddHost(datastore.Get(), &host)
}

func (source *FacadeHostSource) Update(datum sync.Datum) error {
	var host host.Host
	if hd, ok := datum.(zkhost.HostNode); ok {
		host = *hd.Host
	} else {
		return sync.ErrBadType
	}

	return source.facade.UpdateHost(datastore.Get(), &host)
}

func (source *FacadeHostSource) Delete(id string) error {
	if host, err := source.facade.GetHost(datastore.Get(), id); err != nil {
		return err
	} else if host.PoolID != source.poolID {
		return nil
	}

	return source.facade.RemoveHost(datastore.Get(), id)
}

type ZKHostSource struct {
	conn    client.Connection
	timeout time.Duration
}

func (source *ZKHostSource) Init(conn client.Connection, timeout time.Duration) sync.Source {
	return &ZKHostSource{conn, timeout}
}

func (source *ZKHostSource) Get() ([]sync.Datum, error) {
	hosts, err := zkhost.GetHosts(source.conn)
	if err != nil {
		return nil, err
	}

	return ConvertHosts(hosts), nil
}

func (source *ZKHostSource) Add(datum sync.Datum) error {
	var host host.Host
	if hd, ok := datum.(zkhost.HostNode); ok {
		host = *hd.Host
	} else {
		return sync.ErrBadType
	}

	return zkhost.AddHost(source.conn, &host)
}

func (source *ZKHostSource) Update(datum sync.Datum) error {
	var host host.Host
	if hd, ok := datum.(zkhost.HostNode); ok {
		host = *hd.Host
	} else {
		return sync.ErrBadType
	}

	return zkhost.UpdateHost(source.conn, &host)
}

func (source *ZKHostSource) Delete(id string) error {
	mutex := zkhost.ServiceLock(source.conn)
	mutex.Lock()
	defer mutex.Unlock()

	var cancel chan interface{}
	go func() {
		defer close(cancel)
		<-time.After(source.timeout)
	}()

	return zkhost.RemoveHost(cancel, source.conn, id)
}
