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

package dbclient

import (
	"github.com/pkg/errors"

	"github.com/erda-project/erda/internal/tools/orchestrator/components/horizontalpodscaler/types"
	"github.com/erda-project/erda/internal/tools/orchestrator/spec"
	"github.com/erda-project/erda/pkg/database/dbengine"
)

// RuntimeHPA define KEDA ScaledObjects for runtime's service
type RuntimeHPA struct {
	dbengine.BaseModel
	RuleID                 string `json:"rule_id" gorm:"size:36"`
	RuleName               string `json:"rule_name"`
	RuleNameSpace          string `json:"rule_namespace" gorm:"column:rule_namespace"`
	OrgID                  uint64 `json:"org_id" gorm:"not null"`
	OrgName                string `json:"org_name"`
	OrgDisPlayName         string `json:"org_display_name" gorm:"column:org_display_name"`
	ProjectID              uint64 `json:"project_id" gorm:"not null"`
	ProjectName            string `json:"project_name"`
	ProjectDisplayName     string `json:"proj_display_name" gorm:"column:proj_display_name"`
	ApplicationID          uint64 `json:"application_id" gorm:"not null"`
	ApplicationName        string `json:"application_name"`
	ApplicationDisPlayName string `json:"app_display_name" gorm:"column:app_display_name"`
	RuntimeID              uint64 `json:"runtime_id" gorm:"not null"`
	RuntimeName            string `json:"runtime_name"`
	ClusterName            string `json:"cluster_name"` // 部署目标所在 k8s 集群名称
	Workspace              string `json:"workspace" gorm:"column:workspace"`
	UserID                 string `json:"user_id"`   // 操作人ID
	UserName               string `json:"user_name"` // 操作人名称
	NickName               string `json:"nick_name"` // 操作人昵称
	ServiceName            string `json:"service_name"`
	Rules                  string `json:"rules" gorm:"type:text"`
	IsApplied              string `json:"is_applied"` // 表示规则是否已经应用，‘Y’ 表示已经应用，‘N’表示规则存在但未应用
}

const (
	ErdaHPARuleApplied = "DELETING"
)

func (RuntimeHPA) TableName() string {
	return "ps_v2_runtime_hpa"
}

func (db *DBClient) CreateRuntimeHPA(runtimeHPA *RuntimeHPA) error {
	return db.Save(runtimeHPA).Error
}

func (db *DBClient) UpdateRuntimeHPA(runtimeHPA *RuntimeHPA) error {
	if err := db.Model(&RuntimeHPA{}).Where("rule_id = ?", runtimeHPA.RuleID).Update(runtimeHPA).Error; err != nil {
		return errors.Wrapf(err, "failed to update runtime hpa rule, id: %v", runtimeHPA.RuleID)
	}
	return nil
}

// if not found, return (nil, error)
func (db *DBClient) GetRuntimeHPAByServices(id spec.RuntimeUniqueId, services []string) ([]RuntimeHPA, error) {
	var runtimeHPAs []RuntimeHPA
	if len(services) > 0 {
		if err := db.
			Where("application_id = ? AND workspace = ? AND runtime_name = ? AND service_name in (?)", id.ApplicationId, id.Workspace, id.Name, services).
			Find(&runtimeHPAs).Error; err != nil {
			return nil, errors.Wrapf(err, "failed to get runtime hpa rule for runtime %+v for services: %v", id, services)
		}
	} else {
		if err := db.
			Where("application_id = ? AND workspace = ? AND runtime_name = ? ", id.ApplicationId, id.Workspace, id.Name).
			Find(&runtimeHPAs).Error; err != nil {
			return nil, errors.Wrapf(err, "failed to get runtime hpa rule for runtime: %+v", id)
		}
	}
	return runtimeHPAs, nil
}

func (db *DBClient) DeleteRuntimeHPAByRuleId(ruleId string) error {
	if err := db.
		Where("rule_id = ?", ruleId).
		Delete(&RuntimeHPA{}).Error; err != nil {
		return errors.Wrapf(err, "failed to delete runtime hpa rule for rule_id: %v", ruleId)
	}
	return nil
}

func (db *DBClient) GetRuntimeHPARuleByRuleId(ruleId string) (*RuntimeHPA, error) {
	var runtimeHPA RuntimeHPA
	if err := db.
		Where("rule_id = ?", ruleId).
		Find(&runtimeHPA).Error; err != nil {
		return nil, errors.Wrapf(err, "failed to get runtime hpa rule for rule_id: %v", ruleId)
	}
	return &runtimeHPA, nil
}

func (db *DBClient) GetRuntimeHPARulesByRuntimeId(runtimeId uint64) ([]RuntimeHPA, error) {
	var runtimeHPAs []RuntimeHPA
	if err := db.
		Where("runtime_id = ?", runtimeId).
		Find(&runtimeHPAs).Error; err != nil {
		return nil, errors.Wrapf(err, "failed to get runtime hpa rule for runtime_id: %v", runtimeId)
	}
	return runtimeHPAs, nil
}

func ConvertRuntimeHPARuleDTO(runtimeHPA RuntimeHPA) *types.RuntimeHPARuleDTO {
	return &types.RuntimeHPARuleDTO{
		RuleID:          runtimeHPA.RuleID,
		OrgID:           runtimeHPA.OrgID,
		OrgName:         runtimeHPA.OrgName,
		ProjectID:       runtimeHPA.ProjectID,
		ProjectName:     runtimeHPA.ProjectName,
		ApplicationID:   runtimeHPA.ApplicationID,
		ApplicationName: runtimeHPA.ApplicationName,
		RuntimeID:       runtimeHPA.RuntimeID,
		RuntimeName:     runtimeHPA.RuntimeName,
		ClusterName:     runtimeHPA.ClusterName,
		Workspace:       runtimeHPA.Workspace,
		UserId:          runtimeHPA.UserID,
		UserName:        runtimeHPA.UserName,
		NickName:        runtimeHPA.NickName,
		ServiceName:     runtimeHPA.ServiceName,
		Rules:           runtimeHPA.Rules,
		IsApplied:       runtimeHPA.IsApplied,
	}
}
