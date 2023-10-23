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
	"encoding/json"
	"fmt"
)

func GenerateContextPrompt(sceneStepID, sceneID uint64, userID string) string {
	exprValues := getStepExpressionValues(sceneStepID, sceneID, userID)
	//fmt.Println("expression values:")
	//printJSON(exprValues)

	// 构造 prompt
	contextPrompt := fmt.Sprintf(`
场景入参：%s
前置接口出参：%s
前置配置单出参：%s
全局变量入参：%s
mock：%s
`,
		JsonOutput(exprValues.SceneInputs),
		JsonOutput(exprValues.PreSceneStepsOutputs),
		JsonOutput(exprValues.PreConfigSheetsOutputs),
		JsonOutput(exprValues.GlobalConfigOutputs),
		JsonOutput(exprValues.MockInputs),
	)

	return contextPrompt
}

func JsonOutput(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func prettyJsonOutput(s string) string {
	m := make(map[string]interface{})
	json.Unmarshal([]byte(s), &m)
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}
