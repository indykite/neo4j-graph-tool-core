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
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"

	"github.com/indykite/neo4j-graph-tool-core/planner"
)

type Neo4jState string

const (
	boltAddr              = "bolt://127.0.0.1:7687"
	boltCheckSec          = 2
	boltCheckQuitAfterSec = 5 * 60
	importDir             = "/initial-data/import"
	ComponentLogKey       = "system"

	Stopped  Neo4jState = "Stopped"
	Failed   Neo4jState = "Failed"
	Starting Neo4jState = "Starting"
	Stopping Neo4jState = "Stopping"
	Running  Neo4jState = "Running"
)

var (
	processArgs       = []string{"/startup/docker-entrypoint.sh", "neo4j"}
	cancelWaitingChan chan os.Signal
	// Semaphore supports TryAcquire which can checks locks only, and not block execution
	serviceSem = semaphore.NewWeighted(1)
	spinUpMux  = &sync.Mutex{}
	utilsMux   = &sync.Mutex{}
)

type Neo4jWrapper struct {
	serviceCmd   *TSCmd
	log          *logrus.Entry
	utilsCmd     map[string]*TSCmd
	Context      context.Context
	serviceState Neo4jState
}

// NewNeo4jWrapper creates wrapper for handling Neo4j and utilities
func NewNeo4jWrapper(ctx context.Context, log *logrus.Entry) *Neo4jWrapper {
	return &Neo4jWrapper{
		Context:      ctx,
		serviceState: Stopped,
		utilsCmd:     map[string]*TSCmd{},
		log:          log,
	}
}

// State returns the current service state
func (w *Neo4jWrapper) State() (Neo4jState, error) {
	if err := serviceSem.Acquire(w.Context, 1); err != nil {
		return Failed, err
	}
	defer serviceSem.Release(1)
	return w.serviceState, nil
}

// SetState sets the current service state
func (w *Neo4jWrapper) SetState(state Neo4jState) error {
	if err := serviceSem.Acquire(w.Context, 1); err != nil {
		return err
	}
	defer serviceSem.Release(1)
	w.serviceState = state
	return nil
}

// AllStates returns the current states of main service and all utilities
func (w *Neo4jWrapper) AllStates() map[string]interface{} {
	state, err := w.State()
	m := map[string]interface{}{"neo4j": state}
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

// Start the main neo4j process
func (w *Neo4jWrapper) Start() error {
	// Ensure there are no multiple operations running at the same time
	if !serviceSem.TryAcquire(1) {
		return fmt.Errorf("cannot Start service, currently is '%s'", w.serviceState)
	}
	defer serviceSem.Release(1)
	if w.serviceCmd != nil {
		return fmt.Errorf("service is in '%s' state already, cannot be started again", w.serviceState)
	}
	w.log.Trace("Starting neo4j process")
	w.serviceState = Starting
	var err error
	w.serviceCmd, err = StartCmd(w.log.WithField(ComponentLogKey, "neo4j"), nil, processArgs...)
	if err != nil {
		w.serviceState = Failed
		return fmt.Errorf("cannot start neo4j - %v", err.Error())
	}
	w.log.Trace("Process neo4j started")

	// It will set Started state, but no need to wait for it to finish
	go func() { _ = w.WaitForNeo4j() }()

	return nil
}

// Stop the main neo4j process, but does not wait. Use WaitAll() to wait process is exited
// To stop all started processes use StopAll() and WaitAll() optionally
func (w *Neo4jWrapper) Stop() error {
	// Always run Stop and do not fail, so wait until semaphore is released.
	// For example calling stop during starting, it should wait and stop
	if err := serviceSem.Acquire(w.Context, 1); err != nil {
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
			break
		default:
			w.log.Warn("Interrupting signal was sent to Bolt opening checks - channel is full")
			break
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
		if err := serviceSem.Acquire(w.Context, 1); err == nil {
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

// Restart call Stop, Wait and Start
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

// StopAll sends stopping signal for all started processes, but does not wait for exit
func (w *Neo4jWrapper) StopAll() error {
	err := w.Stop()
	if err != nil {
		return err
	}
	utilsMux.Lock()
	defer utilsMux.Unlock()
	for un, v := range w.utilsCmd {
		if v != nil {
			utilsLog(w.log, un).Trace("Sent Interrupt signal")
			err := v.Process.Signal(os.Interrupt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Wait waits until main process is exited
// To wait for utilities use WaitAll
func (w *Neo4jWrapper) Wait() error {
	// Create copy of TSCmd to avoid locking all other methods when waiting
	if err := serviceSem.Acquire(w.Context, 1); err != nil {
		return err
	}
	serviceCmd := w.serviceCmd // nolint:ifshort
	serviceSem.Release(1)

	if serviceCmd != nil {
		_ = serviceCmd.WaitTS()
	}
	w.log.Trace("Neo4j Exited")
	return nil
}

// WaitAll waits until main process and all scripts exit
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
		utilsLog(w.log, n).Debug("Waiting to exit")
		_ = v.WaitTS()
		utilsLog(w.log, n).Trace("Exited")
	}

	return nil
}

func (w *Neo4jWrapper) Driver() (neo4j.Driver, error) {
	return neo4j.NewDriver(boltAddr, neo4j.BasicAuth(getNeo4jBasicAuth()))
}

// WaitForNeo4j blocks execution until Neo4j is ready, or returns error if service is not starting
// Also returns error after 5 minutes of trying to wait
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
	if isStarting {
		if err := serviceSem.Acquire(w.Context, 1); err != nil {
			return err
		}
		cancelWaitingChan = make(chan os.Signal, 1)
		serviceSem.Release(1)
		ticker := time.NewTicker(boltCheckSec * time.Second)
		connected := false
		w.log.Trace("Starting Bolt checking loop")
		dr, err := neo4j.NewDriver(boltAddr, neo4j.BasicAuth(getNeo4jBasicAuth()))
		if err != nil {
			return err
		}
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
				if err := dr.VerifyConnectivity(); err == nil {
					connected = true
					break outerLoop
				}
				w.log.Tracef("Bolt port is not ready yet, waiting %d seconds", boltCheckSec)
			}
		}
		if err := serviceSem.Acquire(w.Context, 1); err != nil {
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

// RefreshData imports all data from schema import folder
func (w *Neo4jWrapper) RefreshData(target *planner.GraphState, dryRun, clean bool, kind planner.Kind) error {
	err := w.update(target, dryRun, clean, kind)
	if err != nil {
		return fmt.Errorf("importing data failed: %v", err)
	}
	return nil
}

const (
	modelVersionCypher = `MATCH (sm:ModelVersion) RETURN sm.version AS version, sm.file AS rev, sm.dirty AS dirty
ORDER BY COALESCE(sm.ts, datetime({year: 0})) DESC, sm.version DESC LIMIT 1`
	dataVersionCypher = `MATCH (sm:DataVersion)   RETURN sm.version AS version, sm.file AS rev, sm.dirty AS dirty
ORDER BY COALESCE(sm.ts, datetime({year: 0})) DESC, sm.version DESC LIMIT 1`
	perfVersionCypher = `MATCH (sm:PerfVersion)   RETURN sm.version AS version, sm.file AS rev, sm.dirty AS dirty
ORDER BY COALESCE(sm.ts, datetime({year: 0})) DESC, sm.version DESC LIMIT 1`
)

func Version(driver neo4j.Driver) (*planner.GraphModel, error) {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	model, err := queryVersion(session, modelVersionCypher)
	if err != nil {
		return nil, err
	}
	data, err := queryVersion(session, dataVersionCypher)
	if err != nil {
		return nil, err
	}
	perf, err := queryVersion(session, perfVersionCypher)
	if err != nil {
		return nil, err
	}
	return &planner.GraphModel{
		Model: model,
		Data:  data,
		Perf:  perf,
	}, nil
}

func queryVersion(session neo4j.Session, cypher string) (*planner.GraphState, error) {
	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		result, err := tx.Run(cypher, nil)
		if err != nil {
			return nil, err
		}
		if result.Next() {
			if result.Err() != nil {
				return nil, result.Err()
			}
			record := result.Record()

			gs := new(planner.GraphState)

			for i, name := range record.Keys {
				switch name {
				case "version":
					v, ok := record.Values[i].(string)
					if !ok {
						return nil, fmt.Errorf("invalid version filed from the response")
					}

					gs.Version, err = semver.NewVersion(v)
					if err != nil {
						return nil, err
					}
				case "rev":
					v, ok := record.Values[i].(int64)
					if !ok {
						return nil, fmt.Errorf("invalid rev filed from the response")
					}
					gs.Revision = uint64(v)
				case "dirty":
					switch v := record.Values[i].(type) {
					case bool:
						_ = v
					case nil:

					default:
						return nil, fmt.Errorf("invalid dirty filed from the response")
					}
				}
			}
			return gs, nil
		}
		return nil, result.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	mr := result.(*planner.GraphState)
	return mr, nil
}

func (w *Neo4jWrapper) update(target *planner.GraphState, dryRun, clean bool, kind planner.Kind) error {
	state, err := w.State()
	if err != nil {
		return err
	} else if state != Running {
		return fmt.Errorf("cannot run import when service is '%s', must be running", w.serviceState)
	}

	var model *planner.GraphModel

	if !clean {
		var d neo4j.Driver
		d, err = w.Driver()
		if err != nil {
			return err
		}
		defer func() { _ = d.Close() }()
		model, err = Version(d)
		if err != nil {
			return err
		}
	}

	t, err := planner.NewScanner(importDir)
	if err != nil {
		return err
	}
	v, err := t.ScanGraphModel()
	if err != nil {
		return err
	}
	v, err = t.ScanData(v)
	if err != nil {
		return err
	}
	v, err = t.ScanPerfData(v)
	if err != nil {
		return err
	}

	execSteps := new(planner.ExecutionSteps)
	if clean {
		if err = drop(importDir, execSteps); err != nil {
			return err
		}
	}

	changed, err := v.Plan(model, target, kind, planner.CreatePlan(execSteps, true))
	if err != nil {
		return err
	}
	if changed != nil && model != nil && changed.Compare(model.Model) < 0 {
		// It's a downgrade
		planner.SetVersion(execSteps, changed)
	}

	if dryRun && changed != nil {
		fmt.Print(execSteps.String())
		return nil
	} else if changed == nil {
		return nil
	}

	user, pass := getNeo4jAuthForCLI()

	if err = w.SetState(Starting); err != nil {
		return err
	}

	for _, step := range *execSteps {
		if step.IsCypher() {
			_, err = w.startUtility(true, step.Cypher(),
				"cypher-shell", /*"--non-interactive",*/
				"--fail-fast", "--format", "verbose",
				"-u", user, "-p", pass, "-d", "neo4j")
		} else {
			toExec := step.Command()
			switch toExec[0] {
			case "exit":
				continue
			case "graph-tool":
				toExec[0] = "/app/graph-tool"
			}
			_, err = w.startUtility(true, nil, append(toExec, "-u", user, "-p", pass)...)
		}
		if err != nil {
			w.log.Warnf("Failed to import file: %v", err)
			break
		}
	}
	w.log.Debug("Import finished file")

	if stateErr := w.SetState(Running); stateErr != nil {
		return stateErr
	}
	return err
}

func (w *Neo4jWrapper) startUtility(wait bool, stdin io.Reader, args ...string) (*TSCmd, error) {
	utilName := args[0]
	utilsMux.Lock()
	if _, found := w.utilsCmd[utilName]; found {
		utilsLog(w.log, utilName).Debug("Utility cannot be started more than once")
		return nil, fmt.Errorf("utility '%s' is already running", utilName)
	}
	utilsLog(w.log, utilName).Trace("Starting utility")
	cmd, err := StartCmd(utilsLog(w.log, utilName), stdin, args...)
	if err != nil {
		return nil, err
	}
	w.utilsCmd[utilName] = cmd
	utilsMux.Unlock()
	utilsLog(w.log, utilName).Trace("Utility started")

	waitAndClean := func() error {
		err = cmd.WaitTS()
		utilsMux.Lock()
		defer utilsMux.Unlock()
		delete(w.utilsCmd, utilName)
		utilsLog(w.log, utilName).Trace("Cleaned up after utility finished")
		return err
	}

	if wait {
		return nil, waitAndClean()
	}

	go func() { _ = waitAndClean() }()
	return cmd, nil
}