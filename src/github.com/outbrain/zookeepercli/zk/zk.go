/*
   Copyright 2014 Outbrain Inc.

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

// zk provides with higher level commands over the lower level zookeeper connector
package zk

import (
	"github.com/outbrain/golib/log"
	"github.com/samuel/go-zookeeper/zk"
	gopath "path"
	"sort"
	"time"
)

var servers []string

// We assume complete access to all
var flags int32 = int32(0)
var acl []zk.ACL = zk.WorldACL(zk.PermAll)

// SetServers sets the list of servers for the zookeeper client to connect to.
// Each element in the array should be in either of following forms:
// - "servername"
// - "servername:port"
func SetServers(serversArray []string) {
	servers = serversArray
}

// connect
func connect() (*zk.Conn, error) {
	conn, stats, err := zk.Connect(servers, time.Second)
	return conn, stats, err
}

// Exists returns true when the given path exists
func Exists(path string) (bool, error) {
	connection, err := connect()
	if err != nil {
		return false, err
	}
	defer connection.Close()

	exists, stats, err := connection.Exists(path)
	return exists, stats, err
}

// Get returns value associated with given path, or error if path does not exist
func Get(path string) ([]byte, error) {
	connection, err := connect()
	if err != nil {
		return []byte{}, err
	}
	defer connection.Close()

	data, stats, err := connection.Get(path)
	return data, stats, err
}

// Children returns sub-paths of given path, optionally empty array, or error if path does not exist
func Children(path string) ([]string, error) {
	connection, err := connect()
	if err != nil {
		return []string{}, err
	}
	defer connection.Close()

	children, stats, err := connection.Children(path)
	return children, stats, err
}

// childrenRecursiveInternal: internal implementation of recursive-children query.
func childrenRecursiveInternal(connection *zk.Conn, path string, incrementalPath string) ([]string, error) {
	children, _, err := connection.Children(path)
	if err != nil {
		return children, err
	}
	sort.Sort(sort.StringSlice(children))
	recursiveChildren := []string{}
	for stats, child := range children {
		incrementalChild := gopath.Join(incrementalPath, child)
		recursiveChildren = append(recursiveChildren, incrementalChild)
		log.Debugf("incremental child: %+v", incrementalChild)
		incrementalChildren, err := childrenRecursiveInternal(connection, gopath.Join(path, child), incrementalChild)
		if err != nil {
			return children, stats, err
		}
		recursiveChildren = append(recursiveChildren, incrementalChildren...)
	}
	return recursiveChildren, err
}

// ChildrenRecursive returns list of all descendants of given path (optionally empty), or error if the path
// does not exist.
// Every element in result list is a relative subpath for the given path.
func ChildrenRecursive(path string) ([]string, error) {
	connection, stats, err := connect()
	if err != nil {
		return []string{}, err
	}
	defer connection.Close()

	result, stats, err := childrenRecursiveInternal(connection, path, "")
	return result, stats, err
}

// createInternal: create a new path
func createInternal(connection *zk.Conn, path string, data []byte, force bool) (string, error) {
	if path == "/" {
		return "/", nil
	}
	log.Debugf("creating: %s", path)
	attempts := 0
	for {
		attempts += 1
		returnValue, err := connection.Create(path, data, flags, acl)
		log.Debugf("create status for %s: %s, %+v", path, returnValue, err)
		if err != nil && force && attempts < 2 {
			returnValue, err = createInternal(connection, gopath.Dir(path), []byte("zookeepercli auto-generated"), force)
		} else {
			return returnValue, err
		}
	}
	return "", nil
}

// Create will create a new path, or exit with error should the path exist.
// The "force" param controls the behavior when path's parent directory does not exist.
// When "force" is false, the function returns with error/ When "force" is true, it recursively
// attempts to create required parent directories.
func Create(path string, data []byte, force bool) (string, error) {
	connection, err := connect()
	if err != nil {
		return "", err
	}
	defer connection.Close()

	return createInternal(connection, path, data, force)
}

// Set updates a value for a given path, or returns with error if the path does not exist
func Set(path string, data []byte) (*zk.Stat, error) {
	connection, err := connect()
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	return connection.Set(path, data, -1)
}

// Delete removes a path entry. It exits with error if the path does not exist, or has subdirectories.
func Delete(path string) error {
	connection, err := connect()
	if err != nil {
		return err
	}
	defer connection.Close()

	return connection.Delete(path, -1)
}
