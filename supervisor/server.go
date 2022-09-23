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
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/indykite/neo4j-graph-tool-core/planner"
)

type httpServer struct {
	neo4j        *Neo4jWrapper
	log, httpLog logrus.FieldLogger
	srv          *http.Server

	defaultTarget *planner.GraphVersion
	defaultBatch  planner.Batch
}

func runHTTPServer(
	ctx context.Context,
	neo4j *Neo4jWrapper,
	logger logrus.FieldLogger,
	target *planner.GraphVersion,
	batch planner.Batch,
) *httpServer {
	s := &httpServer{
		neo4j:         neo4j,
		log:           logger,
		httpLog:       logger.WithField(ComponentLogKey, "http"),
		defaultTarget: target,
		defaultBatch:  batch,
	}

	// Prepare HTTP server routes
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(gin.Recovery())
	g.GET("/refresh-data", s.refreshDataHandler(true))
	g.GET("/refresh-data/:version", s.refreshDataHandler(true))
	g.GET("/update-data", s.refreshDataHandler(false))
	g.GET("/update-data/:version", s.refreshDataHandler(false))
	g.GET("/version", s.versionHandler)
	g.GET("/status", s.wrapperStatusHandler)
	g.GET("/start", s.startServiceHandler)
	g.GET("/stop", s.stopServiceHandler)
	g.GET("/restart", s.restartServiceHandler)
	g.NoRoute(s.error404)

	s.srv = &http.Server{
		Addr:              ":8080",
		Handler:           g,
		ReadHeaderTimeout: time.Second * 2,
	}

	go func() {
		// ListenAndServe always returns error. ErrServerClosed on graceful close.
		if err := s.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			s.httpLog.Fatalf("Serve failed: %v", err)
		}
	}()

	return s
}

func (s *httpServer) close() error {
	return s.srv.Close()
}

func (s *httpServer) startServiceHandler(c *gin.Context) {
	s.httpLog.WithField("req", c.Request.RequestURI).Debug("Dispatching request")
	if err := s.neo4j.Start(); err == nil {
		state, stateErr := s.neo4j.State()
		c.JSON(http.StatusOK, gin.H{
			"msg":         "Service successfully dispatched for starting",
			"neo4j_state": state,
			"state_err":   stateErr,
		})
	} else {
		s.httpLog.WithField("req", c.Request.RequestURI).Warn(err.Error())
		s.sendError(c, err)
	}
}

func (s *httpServer) stopServiceHandler(c *gin.Context) {
	s.httpLog.WithField("req", c.Request.RequestURI).Debug("Dispatching request")
	if err := s.neo4j.Stop(); err == nil {
		state, stateErr := s.neo4j.State()
		c.JSON(http.StatusOK, gin.H{
			"msg":         "Interrupt signal sent",
			"neo4j_state": state,
			"state_err":   stateErr,
		})
	} else {
		s.httpLog.WithField("req", c.Request.RequestURI).Warn(err.Error())
		s.sendError(c, err)
	}
}

func (s *httpServer) restartServiceHandler(c *gin.Context) {
	s.httpLog.WithField("req", c.Request.RequestURI).Debug("Dispatching request")
	if err := s.neo4j.Restart(); err == nil {
		state, stateErr := s.neo4j.State()
		c.JSON(http.StatusOK, gin.H{
			"msg":         "Service successfully dispatched for restart",
			"neo4j_state": state,
			"state_err":   stateErr,
		})
	} else {
		s.httpLog.WithField("req", c.Request.RequestURI).Warn(err.Error())
		s.sendError(c, err)
	}
}

func (s *httpServer) wrapperStatusHandler(c *gin.Context) {
	s.httpLog.WithField("req", c.Request.RequestURI).Debug("Dispatching request")
	code := http.StatusServiceUnavailable
	if state, _ := s.neo4j.State(); state == Running {
		code = http.StatusOK
	}
	c.JSON(code, gin.H{"state": s.neo4j.AllStates()})
}

func (s *httpServer) refreshDataHandler(clean bool) func(*gin.Context) {
	return func(c *gin.Context) {
		s.httpLog.WithField("req", c.Request.RequestURI).Debug("Dispatching request")
		gs, err := s.parseTargetParams(c)
		if err != nil {
			return
		}
		dryRun := false
		if v, ok := c.GetQuery("dryRun"); ok && v == "true" {
			dryRun = true
		}

		loadBatch := s.defaultBatch
		if v, ok := c.GetQuery("batch"); ok {
			loadBatch = planner.Batch(v)
		}
		if err := s.neo4j.RefreshData(gs, dryRun, clean, loadBatch); err == nil {
			c.JSON(http.StatusOK, gin.H{
				"msg": "Data successfully refreshed",
			})
		} else {
			s.httpLog.WithField("req", c.Request.RequestURI).Warn(err.Error())
			s.sendError(c, err)
		}
	}
}

func (s *httpServer) versionHandler(c *gin.Context) {
	// config is validated in supervisor
	p, _ := planner.NewPlanner(s.neo4j.cfg)
	driver, err := s.neo4j.Driver()
	if err != nil {
		s.sendError(c, err)
		return
	}
	defer func() { _ = driver.Close() }()
	model, err := p.Version(driver)
	if err != nil {
		s.sendError(c, err)
		return
	}
	c.JSON(http.StatusOK, model)
}

func (s *httpServer) error404(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "error": "Not found"})
}

func (s *httpServer) sendError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "error": err.Error()})
}

func (s *httpServer) parseTargetParams(c *gin.Context) (*planner.GraphVersion, error) {
	version := c.Param("version")
	if version == "" {
		return s.defaultTarget, nil
	}
	gVer, err := planner.ParseGraphVersion(version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version: " + err.Error()})
		return nil, err
	}
	return gVer, nil
}
