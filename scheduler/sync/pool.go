package sync

import (
	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/datastore"
	"github.com/control-center/serviced/domain/pool"
	"github.com/control-center/serviced/facade"
	"github.com/control-center/serviced/sync"
	zkpool "github.com/control-center/serviced/zzk/service"
)

func ConvertResourcePools(pools []pool.ResourcePool) []sync.Datum {
	data := make([]sync.Datum, len(pools))
	for i := range pools {
		data[i] = zkpool.PoolNode{ResourcePool: &pools[i]}
	}
	return data
}

type FacadePoolSource struct {
	facade *facade.Facade
	realm  string
}

func (source *FacadePoolSource) Init(facade *facade.Facade, realm string) sync.Source {
	return &FacadePoolSource{facade, realm}
}

func (source *FacadePoolSource) Get() ([]sync.Datum, error) {
	pools, err := source.facade.GetResourcePools(datastore.Get())
	if err != nil {
		return nil, err
	}

	return ConvertResourcePools(pools), nil
}

func (source *FacadePoolSource) Add(datum sync.Datum) error {
	var pool pool.ResourcePool
	if pd, ok := datum.(zkpool.PoolNode); ok {
		pool = *pd.ResourcePool
	} else {
		return sync.ErrBadType
	}
	return source.facade.AddResourcePool(datastore.Get(), &pool)
}

func (source *FacadePoolSource) Update(datum sync.Datum) error {
	var pool pool.ResourcePool
	if pd, ok := datum.(zkpool.PoolNode); ok {
		pool = *pd.ResourcePool
	} else {
		return sync.ErrBadType
	}
	return source.facade.UpdateResourcePool(datastore.Get(), &pool)
}

func (source *FacadePoolSource) Delete(id string) error {
	if pool, err := source.facade.GetResourcePool(datastore.Get(), id); err != nil {
		return err
	} else if pool.Realm != source.realm {
		return nil
	}

	// Delete the hosts first
	hosts, err := source.facade.FindHostsInPool(datastore.Get(), id)
	if err != nil {
		return err
	}
	for _, host := range hosts {
		if err := source.facade.RemoveHost(datastore.Get(), host.ID); err != nil {
			return err
		}
	}

	return source.facade.RemoveResourcePool(datastore.Get(), id)
}

type ZKPoolSource struct {
	conn client.Connection
}

func (source *ZKPoolSource) Init(conn client.Connection) sync.Source {
	return &ZKPoolSource{conn}
}

func (source *ZKPoolSource) Get() ([]sync.Datum, error) {
	pools, err := zkpool.GetResourcePools(source.conn)
	if err != nil {
		return nil, err
	}

	return ConvertResourcePools(pools), nil
}

func (source *ZKPoolSource) Add(datum sync.Datum) error {
	var pool pool.ResourcePool
	if pd, ok := datum.(zkpool.PoolNode); ok {
		pool = *pd.ResourcePool
	} else {
		return sync.ErrBadType
	}

	return zkpool.AddResourcePool(source.conn, &pool)
}

func (source *ZKPoolSource) Update(datum sync.Datum) error {
	var pool pool.ResourcePool
	if pd, ok := datum.(zkpool.PoolNode); ok {
		pool = *pd.ResourcePool
	} else {
		return sync.ErrBadType
	}

	return zkpool.UpdateResourcePool(source.conn, &pool)
}

func (source *ZKPoolSource) Delete(id string) error {
	return zkpool.RemoveResourcePool(source.conn, id)
}
