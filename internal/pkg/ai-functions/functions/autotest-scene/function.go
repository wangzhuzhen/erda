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

package autotest_scene

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/erda-project/erda-proto-go/apps/aifunction/pb"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/internal/core/openapi/legacy/component-protocol/pkg/autotest/step"
	"github.com/erda-project/erda/internal/pkg/ai-functions/functions"
	"github.com/erda-project/erda/internal/pkg/ai-functions/sdk"
	"github.com/erda-project/erda/pkg/strutil"
)

const Name = "create-autotest-scene"

//go:embed schema.yaml
var Schema json.RawMessage

//go:embed system-message.txt
var systemMessage string

//go:embed user-message.txt
var userMessage string

var (
	_ functions.Function = (*Function)(nil)
)

func init() {
	functions.Register(Name, New)
}

type Function struct {
	background *pb.Background
}

// FunctionParams 解析 *pb.ApplyRequest 字段 FunctionParams
type FunctionParams struct {
	//AutoTestSceneID uint64          `json:"autoTestSceneID,omitempty"`
	//SystemPrompt    string          `json:"systemPrompt,omitempty"`
	Requirements []AutoTestSceneParam `json:"requirements,omitempty"`
}

type AutoTestSceneParam struct {
	OperationID uint64                           `json:"operationID,omitempty"`
	Prompt      string                           `json:"prompt,omitempty"`
	Req         *apistructs.AutotestSceneRequest `json:"autoTestSceneCreateReq,omitempty"`

	APIs  InputAPIs           `json:"apis,omitempty"`
	Scene OutPutAutoTestScene `json:"scene,omitempty"`
}

// InputAPIs 表示从 API 集市获取的对应需要自动化生成的接口的 APIs
type InputAPIs struct {
	AssetID   string `json:"apiAssetId,omitempty"`   // dice_api_assets 表对应的 asset_id 字段
	VersionID uint64 `json:"apiVersionId,omitempty"` // dice_api_asset_versions 表对应的 id 字段

	// APIIndexIDs 未指定则是所有接口都要生成
	// APIIndexIDs 指定为多个时，其索引顺序为最终生成的场景步骤的顺序（索引小的 API 自动测试时先执行）
	APIIndexIDs []uint64 `json:"apiIndexIds,omitempty"` // dice_api_oas3_index 表对应的 id 字段, 未指定则是所有接口都要生成

	// OperationIDs 未指定则是所有接口都要生成
	// OperationIDs 指定为多个时，其索引顺序为最终生成的场景步骤的顺序（索引小的 API 自动测试时先执行）
	APIOperationIDs []string `json:"apiOperationIds,omitempty"` // dice_api_oas3_index 表对应的  operation_id 字段, 未指定则是所有接口都要生成
}

type OutPutAutoTestScene struct {
	// AutoTestSpaceId 生成的自动化测试接口存储的目标 Space, 如果未指定，则创建名称为 AI_Generated_{asset_id} 的空间
	AutoTestSpaceId uint64 `json:"autotestSpaceId,omitempty"` // dice_autotest_space 表对应的 id 字段

	// AutoTestSceneSetId 生成的自动化测试接口存储的目标场景集, 如果未指定，则创建名称为 AI_Generated_{asset_id} 的场景集
	AutoTestSceneSetId uint64 `json:"autotestSceneSetId,omitempty"` // dice_autotest_scene_set 表对应的 id 字段

	// AutoTestSceneSetId 生成的自动化测试接口存储的目标场景, 如果未指定，则创建名称为 AI_Generated_{asset_id} 的场景集
	AutoTestSceneId uint64 `json:"autotestSceneId,omitempty"` // dice_autotest_scene
}

// AutoTestSceneFunctionInput 用于为单个 API （path + method）生成自动化测试接口的输入
type AutoTestSceneFunctionInput struct {
	AssetID    string
	VersionID  uint64
	APIIndexID uint64

	// API summary
	APIOperationID uint64
	APIName        string
	APIMethod      string
	APIUrl         string

	// user who create or update autotest scene step
	UserID string

	AutoTestSpaceName    string
	AutoTestSpaceID      uint64
	AutoTestSceneSetName string
	AutoTestSceneSetId   uint64
	AutoTestSceneName    string
	AutoTestSceneId      uint64

	Prompt string
}

// AutoTestSceneMeta 用于关联生成的测试接口场景与对应的 API
type AutoTestSceneMeta struct {
	Req                  apistructs.AutotestSceneRequest `json:"autotestSceneCreateReq,omitempty"` // 当前项目 ID 对应的创建测试用例请求
	AutoTestSpaceName    string                          `json:"autotestSpaceName,omitempty"`      // 创建测试接口存储的目标测试空间的名称 ID
	AutoTestSpaceID      uint64                          `json:"autotestSpaceID,omitempty"`        // 创建测试接口存储的目标测试空间的 ID
	AutoTestSceneSetName string                          `json:"autotestSceneSetName,omitempty"`   // 创建测试接口存储的目标测试空间下的 场景集的名称
	AutoTestSceneSetId   uint64                          `json:"autotestSceneSetID,omitempty"`     // 创建测试接口存储的目标测试空间下的 场景集的ID
	AutoTestSceneName    string                          `json:"autotestSceneName,omitempty"`      // 创建测试接口存储的目标测试空间下的 场景集 中对应的场景的名称
	AutoTestSceneId      uint64                          `json:"autotestSceneID,omitempty"`        // 创建测试接口存储的目标测试空间下的 场景集 中对应的场景的 ID
	AutoTestSceneStepId  uint64                          `json:"autotestSceneStepID,omitempty"`    // 创建测试用例成功返回的测试用例 ID
}

func New(ctx context.Context, prompt string, background *pb.Background) functions.Function {
	return &Function{background: background}
}

func (f *Function) Name() string {
	return Name
}

func (f *Function) Description() string {
	return "create autotest scene"
}

func (f *Function) SystemMessage() string {
	return systemMessage
}

func (f *Function) UserMessage() string {
	return userMessage
}

func (f *Function) Schema() (json.RawMessage, error) {
	schema, err := strutil.YamlOrJsonToJson(Schema)
	return schema, err
}

func (f *Function) RequestOptions() []sdk.RequestOption {
	return []sdk.RequestOption{
		sdk.RequestOptionWithResetAPIVersion("2023-07-01-preview"),
	}
}

func (f *Function) CompletionOptions() []sdk.PatchOption {
	return []sdk.PatchOption{
		sdk.PathOptionWithModel("gpt-35-turbo-16k"),
		// 改变温度参数会改变模型的输出。 温度参数可以设置为 0 到 2。 较高的值（例如 0.7）将使输出更随机，并产生更多发散的响应，而较小的值（例如 0.2）将使输出更加集中和具体。
		sdk.PathOptionWithTemperature(0.5),
	}
}

func (f *Function) Callback(ctx context.Context, arguments json.RawMessage, input interface{}, needAdjust bool) (any, error) {
	autotestSceneInput, ok := input.(AutoTestSceneFunctionInput)
	if !ok {
		err := errors.Errorf("input %v with type %T is not valid for AI Function %s", input, input, Name)
		return nil, errors.Wrap(err, "bad request: invalid input")
	}

	bdl := bundle.New(bundle.WithErdaServer())
	var apiInfo apistructs.APIInfoV2
	if err := json.Unmarshal(arguments, &apiInfo); err != nil {
		return nil, errors.Wrap(err, "Unmarshal arguments to APIInfoV2 failed")
	}
	apiInfo.Name = autotestSceneInput.APIName
	apiInfo.Method = autotestSceneInput.APIMethod
	apiInfo.URL = autotestSceneInput.APIUrl
	if apiInfo.Body.Type == apistructs.APIBodyTypeApplicationJSON {
		apiInfo.Body.Content = prettyJsonOutput(apiInfo.Body.Content.(string))
	}

	// update api step
	apiStep := step.APISpec{
		APIInfo: apiInfo,
		Loop:    nil,
	}
	req := apistructs.AutotestSceneRequest{
		AutoTestSceneParams: apistructs.AutoTestSceneParams{
			//ID:      sceneStepID,
			SpaceID: autotestSceneInput.AutoTestSpaceID,
		},
		SceneID: autotestSceneInput.AutoTestSceneId,
		Value:   JsonOutput(apiStep),
		IdentityInfo: apistructs.IdentityInfo{
			UserID: autotestSceneInput.UserID,
		},
		APISpecID: autotestSceneInput.APIOperationID,
		Name:      apiStep.APIInfo.Name,
	}
	//stepID, err := bdl.UpdateAutoTestSceneStep(req)

	// 需要调整，则返回创建测试用例的请求 apistructs.TestCaseCreateRequest
	if needAdjust {
		return AutoTestSceneMeta{
			Req:                  req,
			AutoTestSpaceName:    autotestSceneInput.AutoTestSpaceName,
			AutoTestSpaceID:      autotestSceneInput.AutoTestSpaceID,
			AutoTestSceneSetName: autotestSceneInput.AutoTestSceneSetName,
			AutoTestSceneSetId:   autotestSceneInput.AutoTestSceneSetId,
			AutoTestSceneName:    autotestSceneInput.AutoTestSceneName,
			AutoTestSceneId:      autotestSceneInput.AutoTestSceneId,
		}, nil
	}

	// 无需调整，则返回创建测试用例的请求 apistructs.TestCaseCreateRequest 以及创建成功之后对应的 testcaseID
	aiCreateTestCaseResponse, err := bdl.CreateAutoTestSceneStep(req)
	if err != nil {
		return nil, errors.Wrap(err, "bundle CreateTestCase failed")
	}

	return AutoTestSceneMeta{
		Req:                  req,
		AutoTestSpaceName:    autotestSceneInput.AutoTestSpaceName,
		AutoTestSpaceID:      autotestSceneInput.AutoTestSpaceID,
		AutoTestSceneSetName: autotestSceneInput.AutoTestSceneSetName,
		AutoTestSceneSetId:   autotestSceneInput.AutoTestSceneSetId,
		AutoTestSceneName:    autotestSceneInput.AutoTestSceneName,
		AutoTestSceneId:      autotestSceneInput.AutoTestSceneId,
		AutoTestSceneStepId:  aiCreateTestCaseResponse,
	}, nil
}
