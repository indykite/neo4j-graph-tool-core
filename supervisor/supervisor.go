// Copyright (c) 2023 IndyKite
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
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/migrator"
)

type supervisor struct {
	context   context.Context
	cancelCtx context.CancelFunc
	cfg       *config.Config

	neo4j      *Neo4jWrapper
	log        logrus.FieldLogger
	httpServer *httpServer

	defaultGraphVersion *migrator.TargetVersion
	initialBatch        migrator.Batch
}

// Start the HTTP Supervisor server, Neo4j DB and load initial data.
// Returns error when config is not valid.
func Start(cfg *config.Config) error {
	var err error

	if err = cfg.ValidateWithSupervisor(); err != nil {
		return err
	}
	// Program is checking interrupt channel. But even if it wouldn't, signal.Notify must be here.
	// Otherwise, the program nor the sub proccesses will not receive interrupt signal
	// and docker will kill it immediately
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)

	log := logrus.New()
	log.SetLevel(stringToLogrusLogLevel(cfg.Supervisor.LogLevel))
	log.Formatter = &nested.Formatter{FieldsOrder: []string{componentLogKey}}

	log.Info("Starting supervisor")

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	var neo4j *Neo4jWrapper
	neo4j, err = NewNeo4jWrapper(ctx, cfg, log.WithField(componentLogKey, "wrapper"))
	if err != nil {
		return err
	}

	s := &supervisor{
		context:   ctx,
		cancelCtx: cancelCtx,
		log:       log,
		neo4j:     neo4j,
		cfg:       cfg,
	}

	if err = s.loadBatchTarget(); err != nil {
		return err
	}

	// Start HTTP server in background thread
	s.httpServer = runHTTPServer(neo4j, log, s.defaultGraphVersion, s.initialBatch)

	// Always start Neo4j when supervisor is started
	err = neo4j.Start()
	if err != nil {
		log.Error(err.Error())
	}

	// Will wait for DB and then insert data into DB
	go s.bootstrapDB()

	// All is running, just wait for an interrupt signal to stop
	<-interruptChan
	s.stop()

	return nil
}

func (s *supervisor) loadBatchTarget() error {
	var err error

	if v := s.cfg.Supervisor.DefaultGraphVersion; v != "" {
		s.defaultGraphVersion, err = migrator.ParseTargetVersion(v)
		if err != nil {
			return fmt.Errorf("invalid graph version '%s': %w", v, err)
		}
		s.log.WithField("version", s.defaultGraphVersion).Debug("Target Graph Version is set")
	} else {
		s.log.Warn("Target GraphModel is not set")
	}

	s.initialBatch = migrator.Batch(s.cfg.Supervisor.InitialBatch)
	s.log.WithField("batch", s.cfg.Supervisor.InitialBatch).Debug("Initial batch is set")

	return nil
}

func stringToLogrusLogLevel(level string) logrus.Level {
	l, err := logrus.ParseLevel(level)
	// When invalid level is passed, just set debug and silently ignore the error.
	if err != nil {
		l = logrus.DebugLevel
	}
	return l
}

func (s *supervisor) bootstrapDB() {
	var err error

	err = s.neo4j.WaitForNeo4j()
	if err != nil {
		s.log.WithError(err).Error("service is not available")
		return
	}

	err = s.neo4j.RefreshData(s.defaultGraphVersion, false, true, s.initialBatch)
	if err != nil {
		s.log.WithError(err).Error("failed to bootstrap database")
	}
}

func (s *supervisor) stop() {
	s.log.Debug("Interrupt signal received - Stopping all")
	s.cancelCtx()

	// When closing server, we don't really care about error
	_ = s.httpServer.close()
	var err error
	if s.neo4j.driver != nil {
		err = s.neo4j.driver.Close(s.context)
		if err != nil {
			s.log.Error(err)
		}
	}

	// Send stop all to neo4j first, takes the longest time
	err = s.neo4j.StopAll()
	if err != nil {
		s.log.Error(err)
	}

	err = s.neo4j.WaitAll()
	if err != nil {
		s.log.Error(err)
	}

	// Give time for goroutines to finnish properly
	time.Sleep(1 * time.Millisecond)
	s.log.Info("All quit, good bye")
}
