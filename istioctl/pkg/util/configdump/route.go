// Copyright 2018 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configdump

import (
	"sort"
	"time"

	adminapi "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/golang/protobuf/ptypes"

	v3 "istio.io/istio/pilot/pkg/proxy/envoy/v3"
)

// GetLastUpdatedDynamicRouteTime retrieves the LastUpdated timestamp of the
// most recently updated DynamicRouteConfig
func (w *Wrapper) GetLastUpdatedDynamicRouteTime() (*time.Time, error) {
	routeDump, err := w.GetRouteConfigDump()
	if err != nil {
		return nil, err
	}
	drc := routeDump.GetDynamicRouteConfigs()

	lastUpdated := time.Unix(0, 0) // get the oldest possible timestamp
	for i := range drc {
		if drc[i].LastUpdated != nil {
			if drLastUpdated, err := ptypes.Timestamp(drc[i].LastUpdated); err != nil {
				return nil, err
			} else if drLastUpdated.After(lastUpdated) {
				lastUpdated = drLastUpdated
			}
		}
	}
	if lastUpdated.After(time.Unix(0, 0)) { // if a timestamp was obtained from a drc
		return &lastUpdated, nil
	}
	return nil, nil
}

// GetDynamicRouteDump retrieves a route dump with just dynamic active routes in it
func (w *Wrapper) GetDynamicRouteDump(stripVersions bool) (*adminapi.RoutesConfigDump, error) {
	routeDump, err := w.GetRouteConfigDump()
	if err != nil {
		return nil, err
	}
	drc := routeDump.GetDynamicRouteConfigs()
	sort.Slice(drc, func(i, j int) bool {
		// Support v2 or v3 in config dump. See ads.go:RequestedTypes for more info.
		r := &route.RouteConfiguration{}
		drc[i].RouteConfig.TypeUrl = v3.RouteType
		drc[j].RouteConfig.TypeUrl = v3.RouteType
		err = ptypes.UnmarshalAny(drc[i].RouteConfig, r)
		if err != nil {
			return false
		}
		name := r.Name
		err = ptypes.UnmarshalAny(drc[j].RouteConfig, r)
		if err != nil {
			return false
		}
		return name < r.Name
	})

	// In Istio 1.5, it is not enough just to sort the routes.  The virtual hosts
	// within a route might have a different order.  Sort those too.
	for i := range drc {
		route := &route.RouteConfiguration{}
		err = ptypes.UnmarshalAny(drc[i].RouteConfig, route)
		if err != nil {
			return nil, err
		}
		sort.Slice(route.VirtualHosts, func(i, j int) bool {
			return route.VirtualHosts[i].Name < route.VirtualHosts[j].Name
		})
		drc[i].RouteConfig, err = ptypes.MarshalAny(route)
		if err != nil {
			return nil, err
		}
	}

	if stripVersions {
		for i := range drc {
			drc[i].VersionInfo = ""
			drc[i].LastUpdated = nil
		}
	}
	return &adminapi.RoutesConfigDump{DynamicRouteConfigs: drc}, nil
}

// GetRouteConfigDump retrieves the route config dump from the ConfigDump
func (w *Wrapper) GetRouteConfigDump() (*adminapi.RoutesConfigDump, error) {
	routeDumpAny, err := w.getSection(routes)
	if err != nil {
		return nil, err
	}
	routeDump := &adminapi.RoutesConfigDump{}
	err = ptypes.UnmarshalAny(&routeDumpAny, routeDump)
	if err != nil {
		return nil, err
	}
	return routeDump, nil
}
