// Copyright 2021 PingCAP, Inc.
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

package cte_test

import (
	"runtime"
	"testing"

	"github.com/pingcap/tidb/pkg/domain"
	"github.com/pingcap/tidb/pkg/kv"
	"github.com/pingcap/tidb/pkg/session"
	sessiontypes "github.com/pingcap/tidb/pkg/session/types"
	"github.com/pingcap/tidb/pkg/store/mockstore"
	"github.com/pingcap/tidb/pkg/testkit/testsetup"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/tests/v3/integration"
	"go.opencensus.io/stats/view"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	testsetup.SetupForCommonTest()

	opts := []goleak.Option{
		goleak.IgnoreTopFunction("go.etcd.io/etcd/client/pkg/v3/logutil.(*MergeLogger).outputLoop"),
		goleak.IgnoreTopFunction("github.com/golang/glog.(*fileSink).flushDaemon"),
		goleak.IgnoreTopFunction("github.com/lestrrat-go/httprc.runFetchWorker"),
		goleak.IgnoreTopFunction("syscall.Syscall"),
	}

	goleak.VerifyTestMain(m, opts...)
}

// TestCTEPreviewAndReport requires a separated binary
func TestCTEPreviewAndReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("integration.NewClusterV3 will create file contains a colon which is not allowed on Windows")
	}
	integration.BeforeTestExternal(t)

	s := newSuite(t)
	defer s.close()

	// By disableing telemetry by default, the global sysvar **and** config file defaults
	// are all set to false, so that enabling telemetry in test become more complex.
	// As telemetry is a feature that almost no user will manually enable, I'd remove these
	// tests for now.
	// They should be uncommented once the default behavious changed back to enabled in the
	// future, otherwise they could just be deleted.
	/*
		config.GetGlobalConfig().EnableTelemetry = true

		tk := testkit.NewTestKit(t, s.store)
		tk.MustExec("use test")
		tk.MustExec("with cte as (select 1) select * from cte")
		tk.MustExec("with recursive cte as (select 1) select * from cte")
		tk.MustExec("with recursive cte(n) as (select 1 union select * from cte where n < 5) select * from cte")
		tk.MustExec("select 1")

		res, err := telemetry.PreviewUsageData(s.se, s.etcdCluster.RandClient())
		require.NoError(t, err)

		jsonParsed, err := gabs.ParseJSON([]byte(res))
		require.NoError(t, err)
		require.Equal(t, 1, int(jsonParsed.Path("featureUsage.cte.nonRecursiveCTEUsed").Data().(float64)))
		require.Equal(t, 1, int(jsonParsed.Path("featureUsage.cte.recursiveUsed").Data().(float64)))
		require.Equal(t, 4, int(jsonParsed.Path("featureUsage.cte.nonCTEUsed").Data().(float64)))

		err = telemetry.ReportUsageData(s.se, s.etcdCluster.RandClient())
		require.NoError(t, err)

		res, err = telemetry.PreviewUsageData(s.se, s.etcdCluster.RandClient())
		require.NoError(t, err)

		jsonParsed, err = gabs.ParseJSON([]byte(res))
		require.NoError(t, err)
		require.Equal(t, 0, int(jsonParsed.Path("featureUsage.cte.nonRecursiveCTEUsed").Data().(float64)))
		require.Equal(t, 0, int(jsonParsed.Path("featureUsage.cte.recursiveUsed").Data().(float64)))
		require.Equal(t, 0, int(jsonParsed.Path("featureUsage.cte.nonCTEUsed").Data().(float64)))
	*/
}

type testSuite struct {
	store       kv.Storage
	dom         *domain.Domain
	etcdCluster *integration.ClusterV3
	se          sessiontypes.Session
	close       func()
}

func newSuite(t *testing.T) *testSuite {
	suite := new(testSuite)

	store, err := mockstore.NewMockStore()
	require.NoError(t, err)
	suite.store = store

	session.SetSchemaLease(0)
	session.DisableStats4Test()

	dom, err := session.BootstrapSession(store)
	require.NoError(t, err)
	suite.dom = dom

	etcdCluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	suite.etcdCluster = etcdCluster

	se, err := session.CreateSession4Test(store)
	require.NoError(t, err)
	suite.se = se

	suite.close = func() {
		suite.se.Close()
		suite.etcdCluster.Terminate(t)
		suite.dom.Close()
		err = suite.store.Close()
		require.NoError(t, err)
		view.Stop()
	}

	return suite
}
