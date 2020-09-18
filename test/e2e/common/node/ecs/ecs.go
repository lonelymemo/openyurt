/*
Copyright 2020 The OpenYurt Authors.

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

package ecs

/*
TODO
*/

import (
	"github.com/alibaba/openyurt/test/e2e/common/node/types"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
)

type EcsController struct {
	RegionId string
	Client   *ecs.Client
}

func NewEcsController(regionId, accessKeyId, accessKeySecret string) (*EcsController, error) {
	var e EcsController
	var err error
	e.RegionId = regionId
	e.Client, err = ecs.NewClientWithAccessKey(regionId, accessKeyId, accessKeySecret)
	return &e, err
}

func (e *EcsController) RebootNode(instanceId string) error {
	return nil
}

func (e *EcsController) CreateNode(instanceType, imageId, vswitchId, userData string) (string, error) {
	return "", nil
}

func (e *EcsController) StopNode(instanceId string) error {
	return nil
}

func (e *EcsController) StartNode(instanceId string) error {
	return nil
}

func (e *EcsController) GetNodeInfo(instanceId string) (*types.NodeAttribute, error) {
	return nil, nil
}

func (e *EcsController) DeleteNode(instanceId string) error {
	return nil
}

func (e *EcsController) CheckEcsInstanceStatus(instanceId string, expectStatus string) (bool, error) {
	return false, nil
}
