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
	"io"
	"os"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/migrator"
)

type Neo4jState string

const (
	boltAddr              = "bolt://127.0.0.1:7687"
	boltCheckSec          = 2
	boltCheckQuitAfterSec = 5 * 60
	initialDataDir        = "/initial-data/"
	componentLogKey       = "system"
	dockerEntryPointPath  = "/startup/docker-entrypoint.sh"
	graphToolPath         = "/app/graph-tool"

	Stopped  Neo4jState = "Stopped"
	Failed   Neo4jState = "Failed"
	Starting Neo4jState = "Starting"
	Updating Neo4jState = "Updating Data"
	Stopping Neo4jState = "Stopping"
	Running  Neo4jState = "Running"
)

var (
	processArgs       = []string{dockerEntryPointPath, "neo4j"}
	cancelWaitingChan chan os.Signal
	// Semaphore supports TryAcquire which can checks locks only, and not block execution.
	serviceSem = semaphore.NewWeighted(1)
	spinUpMux  = &sync.Mutex{}
	utilsMux   = &sync.Mutex{}
)

// Neo4jWrapper wraps command and helper functions to operate with Neo4j server together with utilities.
type Neo4jWrapper struct {
	driver  neo4j.DriverWithContext
	context context.Context //nolint:containedctx // Context here is expected and required, it is root context.
	cfg     *config.Config

	serviceCmd   *TSCmd
	log          *logrus.Entry
	utilsCmd     map[string]*TSCmd
	serviceState Neo4jState
}

// NewNeo4jWrapper creates wrapper for handling Neo4j and utilities.
func NewNeo4jWrapper(ctx context.Context, cfg *config.Config, log *logrus.Entry) (*Neo4jWrapper, error) {
	w := &Neo4jWrapper{
		context:      ctx,
		cfg:          cfg,
		serviceState: Stopped,
		utilsCmd:     map[string]*TSCmd{},
		log:          log,
	}
	var err error
	w.driver, err = neo4j.NewDriverWithContext(boltAddr, neo4j.BasicAuth(w.getNeo4jBasicAuth()))

	return w, err
}

// State returns the current service state.
func (w *Neo4jWrapper) State() (Neo4jState, error) {
	if err := serviceSem.Acquire(w.context, 1); err != nil {
		return Failed, err
	}
	defer serviceSem.Release(1)
	return w.serviceState, nil
}

// setState sets the current service state.
func (w *Neo4jWrapper) setState(state Neo4jState) error {
	if err := serviceSem.Acquire(w.context, 1); err != nil {
		return err
	}
	defer serviceSem.Release(1)
	w.serviceState = state
	return nil
}

// AllStates returns the current states of main service and all utilities.
func (w *Neo4jWrapper) AllStates() map[string]any {
	state, err := w.State()
	m := map[string]any{"neo4j": state}
	if err != nil {
		m["neo4_state_err"] = err
	}
	utilsMux.Lock()
	defer utilsMux.Unlock()
	// If the utility is stopped, it will not be in the map anymore
	for k := range w.utilsCmd {
		m[k] = Running
	}
	return m
}

// Start the main neo4j process.
func (w *Neo4jWrapper) Start() error {
	// Ensure there are no multiple operations running at the same time
	if !serviceSem.TryAcquire(1) {
		return fmt.Errorf("cannot Start service, currently is '%s'", w.serviceState)
	}
	defer serviceSem.Release(1)
	if w.serviceCmd != nil {
		return fmt.Errorf("service is in '%s' state already, cannot be started again", w.serviceState)
	}
	w.log.Debug("Starting neo4j process")
	w.serviceState = Starting
	var err error
	w.serviceCmd, err = StartCmd(w.log.WithField(componentLogKey, "neo4j"), nil, processArgs...)
	if err != nil {
		w.serviceState = Failed
		return fmt.Errorf("cannot start neo4j - %v", err.Error())
	}
	w.log.Trace("Process neo4j started")

	// It will set Started state, but no need to wait for it to finish
	go func() { _ = w.WaitForNeo4j() }()

	return nil
}

// Stop the main neo4j process, but does not wait. Use WaitAll() to wait process is exited.
// To stop all started processes use StopAll() and WaitAll() optionally.
func (w *Neo4jWrapper) Stop() error {
	// Always run Stop and do not fail, so wait until semaphore is released.
	// For example calling stop during starting, it should wait and stop
	if err := serviceSem.Acquire(w.context, 1); err != nil {
		return err
	}
	defer serviceSem.Release(1)

	if w.serviceState == Stopping {
		w.log.Trace("Stop request ignored - service is stopping")
		return nil
	}
	if w.serviceCmd == nil {
		return fmt.Errorf("service cannot be stopped, it is '%s'", w.serviceState)
	}
	if cancelWaitingChan != nil {
		// Prevent from blocking if writing multiple times to same channel. Should never happen, but...
		select {
		case cancelWaitingChan <- os.Interrupt:
			w.log.Trace("Interrupting signal was sent to Bolt opening checks")
		default:
			w.log.Warn("Interrupting signal was sent to Bolt opening checks - channel is full")
		}
	}
	w.log.Trace("Interrupting signal sent")
	err := w.serviceCmd.Process.Signal(os.Interrupt)
	if err != nil {
		return err
	}
	w.serviceState = Stopping

	// Wait only for setting correct state, but not block current thread
	go func() {
		w.log.Trace("Waiting for neo4j to exit to cleanup state")
		_ = w.serviceCmd.WaitTS()
		if err := serviceSem.Acquire(w.context, 1); err == nil {
			w.serviceCmd = nil
			w.serviceState = Stopped
			serviceSem.Release(1)
			w.log.Trace("Cleaned up after neo4j exited")
		} else {
			w.log.Errorf("Cannot clean up after neo4j exited: %v", err)
		}
	}()

	return nil
}

// Restart call Stop, Wait and Start.
func (w *Neo4jWrapper) Restart() error {
	stopErr := w.Stop()

	go func() {
		w.log.Trace("Waiting for neo4j to exit")
		err := w.Wait()
		if err != nil {
			w.log.Errorf("Waiting failed: %v", err)
			return
		}
		err = w.Start()
		if err != nil {
			w.log.Errorf("Failed to restart service: %v", err)
		}
	}()

	return stopErr
}

// StopAll sends Interrupt signal for all processes and then stops main Neo4j process, but does not wait for exit.
func (w *Neo4jWrapper) StopAll() error {
	utilsMux.Lock()
	defer utilsMux.Unlock()
	for un, v := range w.utilsCmd {
		if v != nil {
			w.utilsLog(un).Trace("Sent Interrupt signal")
			err := v.Process.Signal(os.Interrupt)
			if err != nil {
				return err
			}
		}
	}

	return w.Stop()
}

// Wait waits until main process is exited.
// To wait for utilities use WaitAll.
func (w *Neo4jWrapper) Wait() error {
	// Create copy of TSCmd to avoid locking all other methods when waiting
	if err := serviceSem.Acquire(w.context, 1); err != nil {
		return err
	}
	serviceCmd := w.serviceCmd
	serviceSem.Release(1)

	if serviceCmd != nil {
		_ = serviceCmd.WaitTS()
	}
	w.log.Trace("Neo4j Exited")
	return nil
}

// WaitAll waits until main process and all scripts exit.
func (w *Neo4jWrapper) WaitAll() error {
	w.log.Debug("Waiting for all processes to exit")
	_ = w.Wait()
	// Create a copy with lock to avoid deadlocks (goroutine inside startUtility) when waiting for processes to exit
	utils := map[string]*TSCmd{}
	utilsMux.Lock()
	for n, v := range w.utilsCmd {
		utils[n] = v
	}
	utilsMux.Unlock()
	for n, v := range utils {
		ul := w.utilsLog(n)
		ul.Debug("Waiting to exit")
		_ = v.WaitTS()
		ul.Trace("Exited")
	}

	return nil
}

// WaitForNeo4j blocks execution until Neo4j is ready, or returns error if service is not starting.
// Also returns error after 5 minutes of trying to wait.
func (w *Neo4jWrapper) WaitForNeo4j() (err error) {
	// Block the function until Neo4j is ready.
	// Only first call to this method will start net.Dial, others are just waiting
	w.log.Trace("Waiting for Neo4j to spin up")
	spinUpMux.Lock()
	defer spinUpMux.Unlock()

	// Check service lock as well
	var state Neo4jState
	if state, err = w.State(); err != nil {
		return err
	}
	isStarting := state == Starting

	cancelled := false
	if isStarting { //nolint:nestif // TODO fix complexity
		if err := serviceSem.Acquire(w.context, 1); err != nil {
			return err
		}
		cancelWaitingChan = make(chan os.Signal, 1)
		serviceSem.Release(1)
		ticker := time.NewTicker(boltCheckSec * time.Second)
		connected := false
		w.log.Trace("Starting Bolt checking loop")
	outerLoop:
		for i := 0; ; i++ {
			select {
			case <-cancelWaitingChan:
				ticker.Stop()
				cancelled = true
				break outerLoop
			case <-ticker.C:
				if i > (boltCheckQuitAfterSec / boltCheckSec) {
					break outerLoop
				}
				if err := w.driver.VerifyConnectivity(w.context); err == nil {
					connected = true
					break outerLoop
				}
				w.log.Tracef("Bolt port is not ready yet, waiting %d seconds", boltCheckSec)
			}
		}
		if err := serviceSem.Acquire(w.context, 1); err != nil {
			return err
		}
		defer serviceSem.Release(1)
		close(cancelWaitingChan)
		cancelWaitingChan = nil

		if connected {
			w.log.Debug("Bolt port is ready, DB connected")
			w.serviceState = Running
		} else if !cancelled {
			w.log.Warnf("Bolt port is not ready after %d seconds, not checking anymore", boltCheckQuitAfterSec)
			w.serviceState = Failed
		}
	}

	// If cancelled, ignore errors as well
	if w.serviceState == Running || cancelled {
		return nil
	}
	return fmt.Errorf("cannot wait for Neo4j, service is '%s'", w.serviceState)
}

// RefreshData imports all data from schema import folder.
func (w *Neo4jWrapper) RefreshData(
	targetVersion *migrator.TargetVersion,
	dryRun, clean bool,
	batchName migrator.Batch,
) error {
	err := w.update(targetVersion, dryRun, clean, batchName)
	if err != nil {
		return fmt.Errorf("importing data failed: %w", err)
	}
	return nil
}

func (w *Neo4jWrapper) setUpdatingStateWhenRunning() error {
	err := serviceSem.Acquire(w.context, 1)
	if err != nil {
		return err
	}
	defer serviceSem.Release(1)

	if w.serviceState != Running {
		return fmt.Errorf("cannot run import when service is '%s', must be running", w.serviceState)
	}
	w.serviceState = Updating
	return nil
}

// update set Updating state in the beginning, but not at the end.
func (w *Neo4jWrapper) update(
	targetVersion *migrator.TargetVersion,
	dryRun, clean bool,
	batchName migrator.Batch,
) (err error) {
	// This check must be done before setting up the defer below.
	err = w.setUpdatingStateWhenRunning()
	if err != nil {
		return err
	}

	// This defer will happen after return and thus have access to returned value.
	// So program can act accordingly and even change it.
	defer func() {
		if err != nil {
			_ = w.setState(Failed)
		} else {
			err = w.setState(Running)
		}
	}()

	w.log.WithFields(logrus.Fields{
		"clean":  clean,
		"batch":  batchName,
		"dryRun": dryRun,
		"target": targetVersion,
	}).Debug("Refreshing data")

	// We already validated config before
	p, _ := migrator.NewPlanner(w.cfg)

	var dbModel migrator.DatabaseModel
	if !clean {
		w.log.Trace("Connecting to DB to fetch current version")

		session := w.ReadOnlySession(w.context)
		defer func() { _ = session.Close(w.context) }()
		dbModel, err = p.Version(w.context, session)
		if err != nil {
			return err
		}
		w.log.WithField("db_model", dbModel).Trace("DB version fetched")
	}

	scanner, err := p.NewScanner(w.getImportDir())
	if err != nil {
		return err
	}
	w.log.WithField("folder", w.getImportDir()).Trace("Scanning folders")
	lf, err := scanner.ScanFolders()
	if err != nil {
		return err
	}
	execSteps := new(migrator.ExecutionSteps)
	if clean {
		if err = w.drop(execSteps); err != nil {
			return err
		}
	}

	err = p.Plan(lf, dbModel, targetVersion, batchName, p.CreateBuilder(execSteps, true))
	if err != nil {
		return err
	}

	switch {
	case execSteps.IsEmpty():
		w.log.Debug("Nothing to change")
		return nil
	case dryRun:
		fmt.Print(execSteps.String())
		return nil
	}

	user, pass, _ := w.getNeo4jBasicAuth()
	// Set environment variables, because the values might come from config.
	// Those variables are added automatically to starting utility. And cypher-shell accept it.
	// Custom commands should accept it as well in the same way as cypher-shell does.
	_ = os.Setenv("NEO4J_USERNAME", user)
	_ = os.Setenv("NEO4J_PASSWORD", pass)
	if w.cfg.Supervisor.Neo4jDatabase != "" {
		_ = os.Setenv("NEO4J_DATABASE", w.cfg.Supervisor.Neo4jDatabase)
	}

	for _, step := range *execSteps {
		if step.IsCypher() {
			err = w.startUtility(true, step.Cypher(),
				"cypher-shell", "--fail-fast", "--format", w.cfg.Planner.CypherShellFormat)
		} else {
			toExec := step.Command()
			if toExec[0] == "exit" {
				continue
			}

			err = w.startUtility(true, nil, toExec...)
		}
		if err != nil {
			w.log.Warnf("Failed to import file: %v", err)
			break
		}
	}
	w.log.Info("Import finished")

	return nil
}

func (w *Neo4jWrapper) startUtility(wait bool, stdin io.Reader, args ...string) error {
	utilName := args[0]
	utilsMux.Lock()
	ul := w.utilsLog(utilName)
	if _, found := w.utilsCmd[utilName]; found {
		ul.Debug("Utility cannot be started more than once")
		return fmt.Errorf("utility '%s' is already running", utilName)
	}
	ul.Trace("Starting utility")
	cmd, err := StartCmd(ul, stdin, args...)
	if err != nil {
		return err
	}
	w.utilsCmd[utilName] = cmd
	utilsMux.Unlock()
	ul.Trace("Utility started")

	waitAndClean := func() error {
		ul.Trace("Starting to clean up")
		err = cmd.WaitTS()
		utilsMux.Lock()
		defer utilsMux.Unlock()
		delete(w.utilsCmd, utilName)
		ul.Trace("Cleaned up after utility finished")
		return err
	}

	if wait {
		return waitAndClean()
	}

	go func() { _ = waitAndClean() }()
	return nil
}
