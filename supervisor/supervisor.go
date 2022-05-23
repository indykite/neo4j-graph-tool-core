// Copyright (c) 2022 IndyKite
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package supervisor

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"github.com/indykite/neo4j-graph-tool-core/planner"
)

type supervisor struct {
	context   context.Context
	cancelCtx context.CancelFunc

	neo4j      *Neo4jWrapper
	log        logrus.FieldLogger
	httpServer *httpServer

	target *planner.GraphState
	kind   planner.Kind
}

func Start() {
	// Program is checking interrupt channel. But even if it wouldn't, signal.Notify must be here.
	// Otherwise, the program nor the sub proccesses will not receive interrupt signal
	// and docker will kill it immediately
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)

	log := logrus.New()
	log.Formatter = &nested.Formatter{FieldsOrder: []string{ComponentLogKey}}
	log.Info("Starting supervisor")

	ctx, cancelCtx := context.WithCancel(context.Background())
	neo4j := NewNeo4jWrapper(ctx, log.WithField(ComponentLogKey, "wrapper"))

	s := &supervisor{
		context:   ctx,
		cancelCtx: cancelCtx,
		log:       log,
		neo4j:     neo4j,
	}

	s.loadKindTargets()

	// Start HTTP server in background thread
	s.httpServer = runHTTPServer(ctx, neo4j, log, s.target, s.kind)

	// Will wait for DB and then insert data into DB
	go s.bootstrapDB()

	// All is running, just wait for an interrupt signal to stop
	<-interruptChan
	s.stop()
}

func (s *supervisor) loadKindTargets() {
	var err error

	if v := os.Getenv("GRAPH_MODEL_VERSION"); v != "" {
		s.target, err = planner.ParseGraphVersion(v)
		if err != nil {
			s.log.Fatalf("invalid model version: %s - %v", v, err)
		}
		s.log.WithField("version", s.target).Info("Target GraphModel is set")
	} else {
		s.log.Warn("Target GraphModel is not set")
	}

	switch os.Getenv("GRAPH_MODEL_KIND") {
	case "perf":
		s.kind = planner.Perf
	case "model":
		s.kind = planner.Model
	default:
		s.kind = planner.Data
	}
}

func (s *supervisor) bootstrapDB() {
	var err error

	err = s.neo4j.WaitForNeo4j()
	if err != nil {
		s.log.WithError(err).Error("service is not available")
		return
	}

	err = s.neo4j.RefreshData(s.target, false, true, s.kind)
	if err != nil {
		s.log.WithError(err).Error("failed to bootstrap database")
	}
}

func (s *supervisor) stop() {
	s.log.Debug("Interrupt signal received - Stopping all")
	s.cancelCtx()

	// When closing server, we don't really care about error
	_ = s.httpServer.close()

	// Send stop all to neo4j first, takes the longest time
	err := s.neo4j.StopAll()
	if err != nil {
		s.log.Error(err)
	}

	err = s.neo4j.WaitAll()
	if err != nil {
		s.log.Error(err)
	}

	// Give time for goroutines to finnish properly
	time.Sleep(1 * time.Millisecond)
	s.log.Info("All quited, good bye")
}
