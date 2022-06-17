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
	"github.com/jinzhu/gorm"

	"github.com/erda-project/erda-infra/base/logs"
	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda-infra/pkg/transport"
	"github.com/erda-project/erda-proto-go/orchestrator/horizontalpodscaler/pb"
	"github.com/erda-project/erda/internal/tools/orchestrator/events"
	"github.com/erda-project/erda/internal/tools/orchestrator/scheduler/impl/servicegroup"
	"github.com/erda-project/erda/pkg/common/apis"
)

type config struct {
}

// +provider
type provider struct {
	Cfg      *config
	Log      logs.Logger
	Register transport.Register
	//hpscalerService *hpscalerService
	hpscalerService pb.HPScalerServiceServer //`autowired:"erda.orchestrator.horizontalpodscaler.HPScalerService"`

	DB           *gorm.DB             `autowired:"mysql-client"`
	EventManager *events.EventManager `autowired:"erda.orchestrator.events.event-manager"`
}

func (p *provider) Init(ctx servicehub.Context) error {
	p.hpscalerService = NewRuntimeHPScalerService(
		WithBundleService(NewBundleService()),
		WithDBService(NewDBService(p.DB)),
		WithEventManagerService(p.EventManager),
		WithServiceGroupImpl(servicegroup.NewServiceGroupImplInit()),
		//WithClusterSvc(p.ClusterSvc),
	)

	if p.Register != nil {
		pb.RegisterHPScalerServiceImp(p.Register, p.hpscalerService, apis.Options())
	}
	return nil
}

func (p *provider) Provide(ctx servicehub.DependencyContext, args ...interface{}) interface{} {
	switch {
	case ctx.Service() == "erda.orchestrator.horizontalpodscaler.HPScalerService" || ctx.Type() == pb.HPScalerServiceServerType() || ctx.Type() == pb.HPScalerServiceHandlerType():
		return p.hpscalerService
	}
	return p
}

func init() {
	servicehub.Register("erda.orchestrator.horizontalpodscaler", &servicehub.Spec{
		Services: pb.ServiceNames(),
		Types:    pb.Types(),
		OptionalDependencies: []string{
			"erda.orchestrator.events",
			"service-register",
			"mysql",
		},
		Description: "",
		ConfigFunc: func() interface{} {
			return &config{}
		},
		Creator: func() servicehub.Provider {
			return &provider{}
		},
	})
}
