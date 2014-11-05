package sync

import (
	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/domain/pool"
	"github.com/control-center/serviced/sync"
	zkvip "github.com/control-center/serviced/zzk/virtualips"
)

type VirtualIP struct {
	pool.VirtualIP
}

func ConvertVirtualIPs(vips []pool.VirtualIP) []sync.Datum {
	data := make([]sync.Datum, len(vips))
	for i, vip := range vips {
		data[i] = VirtualIP{vip}
	}
	return data
}

func (datum VirtualIP) GetID() string {
	return datum.IP
}

type ZKVirtualIPSource struct {
	conn client.Connection
}

func (source *ZKVirtualIPSource) Init(conn client.Connection) sync.Source {
	return &ZKVirtualIPSource{conn}
}

func (source *ZKVirtualIPSource) Get() ([]sync.Datum, error) {
	vips, err := zkvip.GetVirtualIPs(source.conn)
	if err != nil {
		return nil, err
	}
	return ConvertVirtualIPs(vips), nil
}

func (source *ZKVirtualIPSource) Add(datum sync.Datum) error {
	var virtualIP pool.VirtualIP
	if vd, ok := datum.(VirtualIP); ok {
		virtualIP = vd.VirtualIP
	} else {
		return sync.ErrBadType
	}
	return zkvip.AddVirtualIP(source.conn, &virtualIP)
}

func (source *ZKVirtualIPSource) Update(datum sync.Datum) error {
	return nil
}

func (source *ZKVirtualIPSource) Delete(id string) error {
	return zkvip.RemoveVirtualIP(source.conn, id)
}
