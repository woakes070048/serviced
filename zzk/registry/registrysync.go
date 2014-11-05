// Copyright 2014 The Serviced Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"path"

	"github.com/control-center/serviced/coordinator/client"
	"github.com/control-center/serviced/sync"
)

func ConvertKeys(keys []KeyNode) []sync.Datum {
	data := make([]sync.Datum, len(keys))
	for i, key := range keys {
		data[i] = key
	}
	return data
}

type KeySource struct {
	conn client.Connection
	path string
}

func (source *KeySource) Init(conn client.Connection, path string) sync.Source {
	return &KeySource{conn, path}
}

func (source *KeySource) Get() ([]sync.Datum, error) {
	nodes, err := source.conn.Children(source.path)
	if err != nil {
		return nil, err
	}

	data := make([]sync.Datum, len(nodes))
	for i, key := range nodes {
		var node KeyNode
		if err := source.conn.Get(path.Join(source.path, key), &node); err != nil {
			return nil, err
		}
		data[i] = node
	}

	return data, nil
}

func (source *KeySource) Add(datum sync.Datum) error {
	keypath := path.Join(source.path, datum.GetID())

	key, ok := datum.(KeyNode)
	if !ok {
		return sync.ErrBadType
	}
	key.IsRemote = true
	if err := source.conn.Create(keypath, &key); err != nil {
		return err
	}
	return source.conn.Set(keypath, &key)
}

func (source *KeySource) Update(datum sync.Datum) error {
	keypath := path.Join(source.path, datum.GetID())

	key, ok := datum.(KeyNode)
	if !ok {
		return sync.ErrBadType
	}
	key.IsRemote = true
	if _, err := source.conn.Exists(keypath); err != nil {
		return err
	}

	return source.conn.Set(keypath, &key)
}

func (source *KeySource) Delete(id string) error {
	keypath := path.Join(source.path, id)

	var key KeyNode
	if err := source.conn.Get(keypath, &key); err == client.ErrNoNode || !key.IsRemote {
		return nil
	} else if err != nil {
		return err
	}

	return source.conn.Delete(keypath)
}

func ConvertEndpoints(endpoints []EndpointNode) []sync.Datum {
	data := make([]sync.Datum, len(endpoints))
	for i, ep := range endpoints {
		data[i] = ep
	}
	return data
}

type EndpointSource struct {
	conn client.Connection
	key  string
}

func (source *EndpointSource) KeySource(conn client.Connection) sync.Source {
	return new(KeySource).Init(conn, zkEndpoints)
}

func (source *EndpointSource) Init(conn client.Connection, key string) sync.Source {
	return &EndpointSource{conn, key}
}

func (source *EndpointSource) Get() ([]sync.Datum, error) {
	nodes, err := source.conn.Children(path.Join(zkEndpoints, source.key))
	if err != nil {
		return nil, err
	}

	data := make([]sync.Datum, len(nodes))
	for i, id := range nodes {
		var node EndpointNode
		if err := source.conn.Get(path.Join(zkEndpoints, source.key, id), &node); err != nil {
			return nil, err
		}
		data[i] = node
	}

	return data, nil
}

func (source *EndpointSource) Add(datum sync.Datum) error {
	epath := path.Join(zkEndpoints, source.key, datum.GetID())

	ep, ok := datum.(EndpointNode)
	if !ok {
		return sync.ErrBadType
	}
	if err := source.conn.Create(epath, &ep); err != nil {
		return err
	}
	return source.conn.Set(epath, &ep)
}

func (source *EndpointSource) Update(datum sync.Datum) error {
	epath := path.Join(zkEndpoints, source.key, datum.GetID())

	ep, ok := datum.(EndpointNode)
	if !ok {
		return sync.ErrBadType
	}
	if _, err := source.conn.Exists(epath); err != nil {
		return err
	}
	return source.conn.Set(epath, &ep)
}

func (source *EndpointSource) Delete(id string) error {
	return source.conn.Delete(path.Join(zkEndpoints, source.key, id))
}
