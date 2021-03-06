/*
Copyright 2020 Elotl Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/libkv/store"
	"github.com/elotl/kip/pkg/api"
	"github.com/elotl/kip/pkg/etcd"
	"github.com/elotl/kip/pkg/server/events"
	"github.com/elotl/kip/pkg/util"
	"k8s.io/klog"
)

const (
	LogDirectory            string        = "milpa/logs"
	LogDirectoryPlaceholder string        = "milpa/logs/."
	DefaultLogTTL           time.Duration = 1 * time.Hour
)

type LogRegistry struct {
	etcd.Storer
	codec       api.MilpaCodec
	eventSystem *events.EventSystem
	ttl         time.Duration
}

func makeLogKey(creatorName, logName string) string {
	if creatorName == "" && logName == "" {
		return LogDirectory
	} else if logName == "" {
		return LogDirectory + "/" + creatorName
	} else {
		return LogDirectory + "/" + creatorName + "/" + logName
	}
}

func NewLogRegistry(kvstore etcd.Storer, codec api.MilpaCodec, es *events.EventSystem) *LogRegistry {
	reg := &LogRegistry{kvstore, codec, es, DefaultLogTTL}
	_ = reg.Put(LogDirectoryPlaceholder, []byte("."), nil)
	return reg
}

func (reg *LogRegistry) New() api.MilpaObject {
	return api.NewLogFile()
}

func (reg *LogRegistry) Create(obj api.MilpaObject) (api.MilpaObject, error) {
	log := obj.(*api.LogFile)
	return reg.CreateLog(log)
}

func (reg *LogRegistry) Get(compoundName string) (api.MilpaObject, error) {
	parts := strings.SplitN(compoundName, "/", 1)
	if len(parts) < 2 {
		return nil, fmt.Errorf("Invalid log name")
	}
	return reg.GetLog(parts[0], parts[1])
}

func (reg *LogRegistry) List() (api.MilpaObject, error) {
	return reg.ListLogs("", "")
}

func (reg *LogRegistry) Update(obj api.MilpaObject) (api.MilpaObject, error) {
	log := obj.(*api.LogFile)
	return reg.PutLog(log)
}

func (reg *LogRegistry) Delete(fullPath string) (api.MilpaObject, error) {
	// rule of 3s and we've only done this twice...
	parts := strings.SplitN(fullPath, "/", 1)
	if len(parts) < 2 {
		return nil, fmt.Errorf("Invalid log name")
	}
	return reg.GetLog(parts[0], parts[1])
}

func (reg *LogRegistry) CreateLog(log *api.LogFile) (*api.LogFile, error) {
	// we don't care if the log already exists, overwrite it
	return reg.PutLog(log)
}

// Create or overwrite the log, No updates at this time
func (reg *LogRegistry) PutLog(log *api.LogFile) (*api.LogFile, error) {
	key := makeLogKey(log.ParentObject.Name, log.Name)
	data, err := reg.codec.Marshal(log)
	if err != nil {
		return nil, err
	}
	wo := store.WriteOptions{
		TTL: reg.ttl,
	}
	err = reg.Storer.Put(key, data, &wo)
	if err != nil {
		return nil, util.WrapError(err, "Could not write log to registry")
	}

	newLog, err := reg.GetLog(log.ParentObject.Name, log.Name)
	if err != nil {
		return nil, util.WrapError(err, "Could not get log after creation")
	}
	return newLog, nil
}

func (reg *LogRegistry) GetLog(creatorName, logName string) (*api.LogFile, error) {
	key := makeLogKey(creatorName, logName)
	pair, err := reg.Storer.Get(key)
	if err == store.ErrKeyNotFound {

		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("Error retrieving log from storage: %v", err)
	}
	log := api.NewLogFile()
	err = reg.codec.Unmarshal(pair.Value, log)
	if err != nil {
		return nil, util.WrapError(err, "Error unmarshaling log from storage")
	}
	return log, nil
}

func (reg *LogRegistry) ListLogs(creatorName, logName string) (*api.LogFileList, error) {
	key := makeLogKey(creatorName, logName)
	pairs, err := reg.Storer.List(key)
	loglist := api.NewLogFileList()
	if err != nil {
		klog.Errorf("Error listing logs in storage: %v", err)
		return loglist, err
	}
	for _, pair := range pairs {
		// we create a blank key because dealing with "key does not
		// exist across different DBs is a road we dont want to go
		// down yet
		if pair.Key == LogDirectoryPlaceholder {
			continue
		}
		log := api.NewLogFile()
		err = reg.codec.Unmarshal(pair.Value, log)
		if err != nil {
			klog.Errorf("Error unmarshalling single log in list operation: %v", err)
			continue
		}
		loglist.Items = append(loglist.Items, log)
	}
	return loglist, nil
}
