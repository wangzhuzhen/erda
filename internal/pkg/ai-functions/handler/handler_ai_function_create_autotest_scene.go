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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda-proto-go/apps/aifunction/pb"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/internal/pkg/ai-functions/functions"
	aiautotestsecene "github.com/erda-project/erda/internal/pkg/ai-functions/functions/autotest-scene"
	aiHandlerUtils "github.com/erda-project/erda/internal/pkg/ai-functions/handler/utils"
	"github.com/erda-project/erda/pkg/http/httpserver"
)

const (
	AIGeneratedAutoTestSpaceNamePrefix    = "AI_Generated_Space_"
	AIGeneratedAutoTestSceneSetNamePrefix = "AI_Generated_SceneSet_"
	AIGeneratedAutoTestSceneNamePrefix    = "AI_Generated_Scene_"
)

func (h *AIFunction) createAutoTestSceneForAPIsAndAutoTestSceneID(ctx context.Context, factory functions.FunctionFactory, req *pb.ApplyRequest, openaiURL *url.URL) (any, error) {
	results := make([]any, 0)
	var wg sync.WaitGroup
	var functionParams aiautotestsecene.FunctionParams

	// 查询文档中的接口详情
	// "/api/apim/operations/{id}", Method: http.MethodGet  Handler: e.GetOperation

	// GetAPIAssetVersion 查询 API 资产版本详情
	// "/api/api-assets/{assetID}/versions/{versionID}", Method: http.MethodGet, Handler: e.GetAPIAssetVersion
	// 比如根据 assetID（asset_id） 和 versionID (version_id)  可以查询到 API 详情
	/*
		MySQL [erda]> select * from  dice_api_asset_version_specs  where version_id='537' \G;
		*************************** 1. row ***************************
		           id: 537
		       org_id: 633
		     asset_id: csrf-go-http-demo
		   version_id: 537
	*/

	FunctionParamsBytes, err := req.GetFunctionParams().MarshalJSON()
	if err != nil {
		return nil, errors.Wrapf(err, "MarshalJSON for req.FunctionParams failed.")
	}
	if err = json.Unmarshal(FunctionParamsBytes, &functionParams); err != nil {
		return nil, errors.Wrapf(err, "Unmarshal req.FunctionParams to struct FunctionParams failed.")
	}
	logrus.Debugf("parse createTestCase functionParams=%+v", functionParams)

	if err := validateParamsForCreateAutoTestScene(functionParams); err != nil {
		return nil, errors.Wrapf(err, "validateParamsForCreateAutoTestScene failed")
	}

	orgId := req.GetBackground().GetOrgID()
	projectId := req.GetBackground().GetProjectID()
	userId := req.GetBackground().GetUserID()

	bdl := bundle.New(bundle.WithErdaServer())
	// 用户未指定 自动化测试场景 的 Space ID、Set ID、Scene ID 相关的参数，需要创建对应的对象
	for idx, rs := range functionParams.Requirements {
		switch {
		case rs.Scene.AutoTestSpaceId == 0 && rs.Scene.AutoTestSceneSetId == 0 && rs.Scene.AutoTestSceneId == 0:
			// Space、Set、Scene 都不能确定, 依次创建 Space、Set、Scene
			// 1. 创建 Space
			testSpaceName := AIGeneratedAutoTestSpaceNamePrefix + rs.APIs.AssetID
			space, err := bdl.CreateTestSpaceWithReturn(&apistructs.AutoTestSpaceCreateRequest{
				Name:          testSpaceName,
				ProjectID:     int64(projectId),
				Description:   "AI Generated AutoTest Space",
				ArchiveStatus: apistructs.TestSpaceInit,
			}, userId)
			if err != nil {
				return nil, errors.Wrap(err, "create autotest space failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSpaceId = space.ID

			// 2. 创建 Set
			testSceneSetName := AIGeneratedAutoTestSceneSetNamePrefix + rs.APIs.AssetID
			sceneSetId, err := bdl.CreateSceneSet(apistructs.SceneSetRequest{
				Name:        testSceneSetName,
				Description: "AI Generated AutoTest Scene Set",
				SpaceID:     space.ID,
				ProjectId:   projectId,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "create autotest scene set failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneSetId = *sceneSetId

			// 3. 创建 Scene
			testSceneName := AIGeneratedAutoTestSceneNamePrefix + rs.APIs.AssetID
			testSceneId, err := bdl.CreateAutoTestScene(apistructs.AutotestSceneRequest{
				AutoTestSceneParams: apistructs.AutoTestSceneParams{
					SpaceID: space.ID,
				},
				Name:        testSceneName,
				Description: "AI Generated AutoTest Scene",
				SetID:       *sceneSetId,
				//RefSetID:    *formData.ScenesSet,
				//Policy: apistructs.NewRunPolicyType,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "create autotest scene failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneId = testSceneId

		case rs.Scene.AutoTestSpaceId == 0 && rs.Scene.AutoTestSceneSetId == 0 && rs.Scene.AutoTestSceneId > 0:
			// Scene 确定，无需创建    Space 和 Set ID  可以从 Scene 查询
			// 获取 scene 详情
			scene, err := bdl.GetAutoTestScene(apistructs.AutotestSceneRequest{
				SceneID: rs.Scene.AutoTestSceneId,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "get autotest scene by ID failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSpaceId = scene.SpaceID
			functionParams.Requirements[idx].Scene.AutoTestSceneSetId = scene.SetID

		case rs.Scene.AutoTestSpaceId == 0 && rs.Scene.AutoTestSceneSetId > 0 && rs.Scene.AutoTestSceneId == 0:
			// Set 确定，但需要创建 Scene   Space ID  可以从 Set 查询
			// 1. 获取 Space (根据 Set 中的  space_id) 获取 space 的 ID
			sceneSet, err := bdl.GetSceneSet(apistructs.SceneSetRequest{
				SetID: rs.Scene.AutoTestSceneSetId,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "get autotest scene set by ID failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSpaceId = sceneSet.SpaceID

			// 2. 创建 Scene
			testSceneName := AIGeneratedAutoTestSceneNamePrefix + rs.APIs.AssetID
			testSceneId, err := bdl.CreateAutoTestScene(apistructs.AutotestSceneRequest{
				AutoTestSceneParams: apistructs.AutoTestSceneParams{
					SpaceID: sceneSet.SpaceID,
				},
				Name:        testSceneName,
				Description: "AI Generated AutoTest Scene",
				SetID:       rs.Scene.AutoTestSceneSetId,
				//RefSetID:    *formData.ScenesSet,
				//Policy: apistructs.NewRunPolicyType,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "create autotest scene failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneId = testSceneId

		case rs.Scene.AutoTestSpaceId == 0 && rs.Scene.AutoTestSceneSetId > 0 && rs.Scene.AutoTestSceneId > 0:
			// Set、Scene 确定，无需创建   Space ID  可以从 Set 或 Scene  查询
			// 获取 scene 详情
			scene, err := bdl.GetAutoTestScene(apistructs.AutotestSceneRequest{
				SceneID: rs.Scene.AutoTestSceneId,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "get autotest scene by ID failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSpaceId = scene.SpaceID

		case rs.Scene.AutoTestSpaceId > 0 && rs.Scene.AutoTestSceneSetId == 0 && rs.Scene.AutoTestSceneId == 0:
			// Space 确定, 需要依次创建  Set、Scene
			// 1. 创建 Set
			testSceneSetName := AIGeneratedAutoTestSceneSetNamePrefix + rs.APIs.AssetID
			sceneSetId, err := bdl.CreateSceneSet(apistructs.SceneSetRequest{
				Name:        testSceneSetName,
				Description: "AI Generated AutoTest Scene Set",
				SpaceID:     rs.Scene.AutoTestSpaceId,
				ProjectId:   projectId,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "create autotest scene set failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneSetId = *sceneSetId

			// 2. 创建 Scene
			testSceneName := AIGeneratedAutoTestSceneNamePrefix + rs.APIs.AssetID
			testSceneId, err := bdl.CreateAutoTestScene(apistructs.AutotestSceneRequest{
				AutoTestSceneParams: apistructs.AutoTestSceneParams{
					SpaceID: rs.Scene.AutoTestSpaceId,
				},
				Name:        testSceneName,
				Description: "AI Generated AutoTest Scene",
				SetID:       *sceneSetId,
				//RefSetID:    *formData.ScenesSet,
				//Policy: apistructs.NewRunPolicyType,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "create autotest scene failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneId = testSceneId

		case rs.Scene.AutoTestSpaceId > 0 && rs.Scene.AutoTestSceneSetId == 0 && rs.Scene.AutoTestSceneId > 0:
			// Space、Scene 确定，无需创建   Set ID  可以从 Scene 查询
			// 获取 scene 详情
			scene, err := bdl.GetAutoTestScene(apistructs.AutotestSceneRequest{
				SceneID: rs.Scene.AutoTestSceneId,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "get autotest scene by ID failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneSetId = scene.SetID

		case rs.Scene.AutoTestSpaceId > 0 && rs.Scene.AutoTestSceneSetId > 0 && rs.Scene.AutoTestSceneId == 0:
			// Space、Set 确定，但需要创建 Scene
			// 1. 创建 Scene
			testSceneName := AIGeneratedAutoTestSceneNamePrefix + rs.APIs.AssetID
			testSceneId, err := bdl.CreateAutoTestScene(apistructs.AutotestSceneRequest{
				AutoTestSceneParams: apistructs.AutoTestSceneParams{
					SpaceID: rs.Scene.AutoTestSpaceId,
				},
				Name:        testSceneName,
				Description: "AI Generated AutoTest Scene",
				SetID:       rs.Scene.AutoTestSceneSetId,
				//RefSetID:    *formData.ScenesSet,
				//Policy: apistructs.NewRunPolicyType,
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userId,
				},
			})
			if err != nil {
				return nil, errors.Wrap(err, "create autotest scene failed")
			}
			functionParams.Requirements[idx].Scene.AutoTestSceneId = testSceneId

		default:
			// Space、Set、Scene 确定，无需创建
			// rs.Scene.AutoTestSpaceID > 0 && rs.Scene.AutoTestSceneSetId > 0 && rs.Scene.AutoTestSceneId > 0:

		}
	}

	// TODO: 1. 根据每个 API 生成 场景对应的  入参   2. 根据每个 API 生成每个场景步骤的 出参
	for idx, rs := range functionParams.Requirements {
		for _, apiIndexId := range rs.APIs.APIIndexIDs {

			apiDetail, err := bdl.GetAPIOperation(orgId, userId, apiIndexId)
			if err != nil {
				return nil, errors.Wrap(err, "get api info failed when create SceneInput for creating autotest scene step")
			}
			// 1.1 从参数生成入参
			for _, p := range apiDetail.Parameters {
				_, err = bdl.CreateAutoTestSceneInput(apistructs.AutotestSceneRequest{
					AutoTestSceneParams: apistructs.AutoTestSceneParams{
						//ID:        functionParams.Requirements[idx].Scene.AutoTestSceneId,
						SpaceID:   functionParams.Requirements[idx].Scene.AutoTestSpaceId,
						CreatorID: userId,
						UpdaterID: userId,
					},
					Name:        p.Name,
					Description: "",
					// TODO: WZZ value 如何生成？
					Value: "",
					// TODO: WZZ temp value 如何生成？
					Temp:    "",
					SceneID: functionParams.Requirements[idx].Scene.AutoTestSceneId,
					SetID:   functionParams.Requirements[idx].Scene.AutoTestSceneSetId,
					IdentityInfo: apistructs.IdentityInfo{
						UserID: userId,
					},
				})
				if err != nil {
					return nil, errors.Wrapf(err, "CreateAutoTestSceneInput for API Index ID [dice_api_oas3_index id=%d method=%s path=%s] failed", apiIndexId, apiDetail.Method, apiDetail.Path)
				}
			}
			// 1.2 从 Header 生成入参
			for _, p := range apiDetail.Headers {
				_, err = bdl.CreateAutoTestSceneInput(apistructs.AutotestSceneRequest{
					AutoTestSceneParams: apistructs.AutoTestSceneParams{
						//ID:        functionParams.Requirements[idx].Scene.AutoTestSceneId,
						SpaceID:   functionParams.Requirements[idx].Scene.AutoTestSpaceId,
						CreatorID: userId,
						UpdaterID: userId,
					},
					Name:        p.Name,
					Description: "",
					// TODO: WZZ value 如何生成？
					Value: "",
					// TODO: WZZ temp value 如何生成？
					Temp:    "",
					SceneID: functionParams.Requirements[idx].Scene.AutoTestSceneId,
					SetID:   functionParams.Requirements[idx].Scene.AutoTestSceneSetId,
					IdentityInfo: apistructs.IdentityInfo{
						UserID: userId,
					},
				})
				if err != nil {
					return nil, errors.Wrapf(err, "CreateAutoTestSceneInput for API Index ID [dice_api_oas3_index id=%d method=%s path=%s] failed", apiIndexId, apiDetail.Method, apiDetail.Path)
				}
			}

			// 2 从 Respnose 生成出参
			for _, p := range apiDetail.Responses {
				_, err = bdl.CreateAutoTestSceneOutput(apistructs.AutotestSceneRequest{
					AutoTestSceneParams: apistructs.AutoTestSceneParams{
						//ID:        functionParams.Requirements[idx].Scene.AutoTestSceneId,
						SpaceID:   functionParams.Requirements[idx].Scene.AutoTestSpaceId,
						CreatorID: userId,
						UpdaterID: userId,
					},
					// TODO: WZZ 出参名称如何生成？
					Name:        "statusCode",
					Description: "",
					// TODO: WZZ value 如何生成？
					Value: p.StatusCode,
					// TODO: WZZ temp value 如何生成？
					Temp:    p.StatusCode,
					SceneID: functionParams.Requirements[idx].Scene.AutoTestSceneId,
					SetID:   functionParams.Requirements[idx].Scene.AutoTestSceneSetId,
					IdentityInfo: apistructs.IdentityInfo{
						UserID: userId,
					},
				})
				if err != nil {
					return nil, errors.Wrapf(err, "CreateAutoTestSceneOutput for API Index ID [dice_api_oas3_index id=%d method=%s path=%s] failed", apiIndexId, apiDetail.Method, apiDetail.Path)
				}
			}

		}

	}
	//

	for idx, tsp := range functionParams.Requirements {
		for _, apiIndexId := range tsp.APIs.APIIndexIDs {
			wg.Add(1)
			var err error

			apiDetail, err := bdl.GetAPIOperation(orgId, userId, apiIndexId)
			if err != nil {
				return nil, errors.Wrap(err, "get api info failed when create SceneInput for creating autotest scene step")
			}

			atsfi := aiautotestsecene.AutoTestSceneFunctionInput{
				AssetID:              tsp.APIs.AssetID,
				VersionID:            tsp.APIs.VersionID,
				APIIndexID:           apiIndexId,
				APIOperationID:       apiDetail.ID,
				APIName:              apiDetail.Description,
				APIMethod:            apiDetail.Method,
				APIUrl:               apiDetail.Path,
				UserID:               userId,
				AutoTestSpaceName:    "",
				AutoTestSpaceID:      tsp.Scene.AutoTestSpaceId,
				AutoTestSceneSetName: "",
				AutoTestSceneSetId:   tsp.Scene.AutoTestSceneSetId,
				AutoTestSceneName:    "",
				AutoTestSceneId:      tsp.Scene.AutoTestSceneId,
				// TODO: need Prompt
				Prompt: "",
			}
			// TODO: WZZ  GET sceneStepID
			var sceneStepID uint64
			if err = processSingleAPIAutoTestSceneStep(ctx, factory, req, openaiURL, &wg, tsp, atsfi, "", apiIndexId, orgId, userId, sceneStepID, functionParams.Requirements[idx].Scene.AutoTestSceneId, &results); err != nil {
				return nil, errors.Wrapf(err, "process single testCase create faild")
			}
		}
	}
	wg.Wait()

	content := httpserver.Resp{
		Success: true,
		Data:    results,
	}
	return json.Marshal(content)
}

func processSingleAPIAutoTestSceneStep(ctx context.Context, factory functions.FunctionFactory, req *pb.ApplyRequest, openaiURL *url.URL, wg *sync.WaitGroup, atsp aiautotestsecene.AutoTestSceneParam, callbackInput aiautotestsecene.AutoTestSceneFunctionInput, systemPrompt string, apiIndexID, orgID uint64, userID string, sceneStepID, sceneID uint64, results *[]any) error {
	defer wg.Done()

	bdl := bundle.New(bundle.WithErdaServer())
	// 1. 获取 API 接口详情 （method + url）

	apiDetail, err := bdl.GetAPIOperation(orgID, userID, apiIndexID)
	if err != nil {
		return errors.Wrap(err, "get api info failed when create autotest scene step")
	}

	if atsp.Req != nil {
		// 表示是修改后批量应用生成的测试接口，直接调用创建接口，无需再次生成

		/*
			// update api step
			apiStep := step.APISpec{
				APIInfo: apiInfo,
				Loop:    nil,
			}
			updateReq := apistructs.AutotestSceneRequest{
				AutoTestSceneParams: apistructs.AutoTestSceneParams{
					ID:      sceneStepID,
					SpaceID: spaceID,
				},
				SceneID: sceneID,
				Value:   jsonOutput(apiStep),
				IdentityInfo: apistructs.IdentityInfo{
					UserID: userID,
				},
				APISpecID: apiSpecDetail.ID,
				Name:      apiStep.APIInfo.Name,
			}
			stepID, err := bdl.UpdateAutoTestSceneStep(updateReq)
			if err != nil {
				err = errors.Errorf("create testcase with req %+v failed: %v", tp.Req, err)
				return errors.Wrap(err, "bundle CreateTestCase failed for ")
			}

			*results = append(*results, aiautotestsecene.AutoTestSceneMeta{
				Req:                  apistructs.AutotestSceneRequest{},
				AutoTestSpaceName:    "",
				AutoTestSpaceID:      0,
				AutoTestSceneSetName: "",
				AutoTestSceneSetId:   0,
				AutoTestSceneName:    "",
				AutoTestSceneId:      0,
				AutoTestSceneStepId:  0,
			})
		*/
	} else {

		f := factory(ctx, "", req.GetBackground())
		messages := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: f.SystemMessage(),
				Name:    "system",
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: fmt.Sprintf("The swagger documentation content of the API selected by the user: %s", aiautotestsecene.JsonOutput(apiDetail)),
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: fmt.Sprintf("In this test case generation, you can also use these context variables: %s", aiautotestsecene.GenerateContextPrompt(sceneStepID, sceneID, userID)),
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: f.UserMessage(),
				Name:    "erda",
			},
		}

		result, err := aiHandlerUtils.GetChatMessageFunctionCallArguments(ctx, factory, req, openaiURL, messages, callbackInput)
		if err != nil {
			return err
		}

		*results = append(*results, result)
	}

	return nil
}

// validateParamsForCreateAutoTestScene 校验创建自动化测试场景的请求参数
func validateParamsForCreateAutoTestScene(req aiautotestsecene.FunctionParams) error {
	if len(req.Requirements) == 0 {
		return errors.Errorf("AI function functionParams requirements for %s invalid, not set", aiautotestsecene.Name)
	}

	for idx, acp := range req.Requirements {
		if acp.APIs.AssetID == "" {
			return errors.Errorf("AI function functionParams requirements[%d].APIs.AssetID  for %s invalid, not set", idx, aiautotestsecene.Name)
		}
	}

	return nil
}
