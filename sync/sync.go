package sync

import (
	"fmt"

	"github.com/zenoss/glog"
)

var ErrBadType = fmt.Errorf("invalid type")

type Datum interface {
	GetID() string
}

type Source interface {
	Get() ([]Datum, error)
	Add(Datum) error
	Update(Datum) error
	Delete(string) error
}

func Synchronize(in []Datum, out Source) bool {
	data, err := out.Get()
	if err != nil {
		glog.Errorf("Could not get local data: %s", err)
		return false
	}

	datamap := make(map[string]Datum)
	for _, datum := range data {
		datamap[datum.GetID()] = datum
	}

	ok := true
	for _, datum := range in {
		if _, ok := datamap[datum.GetID()]; ok {
			if err := out.Update(datum); err != nil {
				glog.Errorf("Could not update %s: %s", datum.GetID(), err)
				ok = false
			}
			delete(datamap, datum.GetID())
		} else {
			if err := out.Add(datum); err != nil {
				glog.Errorf("Could not add %s: %s", datum.GetID(), err)
				ok = false
			}
		}
	}

	for id := range datamap {
		if err := out.Delete(id); err != nil {
			glog.Errorf("Could not delete %s: %s", id, err)
			ok = false
		}
	}

	return ok
}
