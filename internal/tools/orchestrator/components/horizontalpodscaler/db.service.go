// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package horizontalpodscaler

import (
	"github.com/erda-project/erda/internal/tools/orchestrator/dbclient"
	"github.com/erda-project/erda/internal/tools/orchestrator/spec"
)

type DBService interface {
	CreateErdaHPARule(req *dbclient.RuntimeHPA) error
	UpdateErdaHPARule(req *dbclient.RuntimeHPA) error
	GetErdaHRuntimePARulesByServices(id spec.RuntimeUniqueId, services []string) ([]dbclient.RuntimeHPA, error)
	GetErdaHRuntimePARuleByRuleId(ruleId string) (dbclient.RuntimeHPA, error)
	GetErdaHRuntimePARulesByRuntimeId(runtimeID uint64) ([]dbclient.RuntimeHPA, error)
	DeleteErdaHRuntimePARulesByRuleId(ruleId string) error
	GetRuntime(id uint64) (*dbclient.Runtime, error)
	GetPreDeployment(uniqueId spec.RuntimeUniqueId) (*dbclient.PreDeployment, error)
}
