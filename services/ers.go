/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package services

import (
	"fmt"
	"sync"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/engine"
	"github.com/cgrates/cgrates/ers"
	"github.com/cgrates/cgrates/servmanager"
	"github.com/cgrates/cgrates/utils"
	"github.com/cgrates/rpcclient"
)

// NewEventReaderService returns the EventReader Service
func NewEventReaderService(cfg *config.CGRConfig, filterSChan chan *engine.FilterS,
	sSChan, dispatcherChan chan rpcclient.RpcClientConnection,
	exitChan chan bool) servmanager.Service {
	return &EventReaderService{
		rldChan:        make(chan struct{}, 1),
		cfg:            cfg,
		filterSChan:    filterSChan,
		sSChan:         sSChan,
		dispatcherChan: dispatcherChan,
		exitChan:       exitChan,
	}
}

// EventReaderService implements Service interface
type EventReaderService struct {
	sync.RWMutex
	cfg            *config.CGRConfig
	filterSChan    chan *engine.FilterS
	sSChan         chan rpcclient.RpcClientConnection
	dispatcherChan chan rpcclient.RpcClientConnection
	exitChan       chan bool

	ers      *ers.ERService
	rldChan  chan struct{}
	stopChan chan struct{}
}

// Start should handle the sercive start
func (erS *EventReaderService) Start() (err error) {
	if erS.IsRunning() {
		return fmt.Errorf("service aleady running")
	}

	erS.Lock()
	defer erS.Unlock()

	filterS := <-erS.filterSChan
	erS.filterSChan <- filterS

	// remake tht stop chan
	erS.stopChan = make(chan struct{}, 1)

	utils.Logger.Info(fmt.Sprintf("<%s> starting <%s> subsystem", utils.CoreS, utils.ERs))
	var sS rpcclient.RpcClientConnection
	if sS, err = NewConnection(erS.cfg, erS.sSChan, erS.dispatcherChan, erS.cfg.ERsCfg().SessionSConns); err != nil {
		utils.Logger.Crit(fmt.Sprintf("<%s> failed connecting to <%s>, error: <%s>",
			utils.ERs, utils.SessionS, err.Error()))
		return
	}
	// build the service
	erS.ers = ers.NewERService(erS.cfg, filterS, sS, erS.stopChan)
	go func(ers *ers.ERService, rldChan chan struct{}) {
		if err := ers.ListenAndServe(rldChan); err != nil {
			utils.Logger.Err(fmt.Sprintf("<%s> error: <%s>", utils.ERs, err.Error()))
			erS.exitChan <- true
		}
	}(erS.ers, erS.rldChan)
	return
}

// GetIntenternalChan returns the internal connection chanel
func (erS *EventReaderService) GetIntenternalChan() (conn chan rpcclient.RpcClientConnection) {
	return nil
}

// Reload handles the change of config
func (erS *EventReaderService) Reload() (err error) {
	var sS rpcclient.RpcClientConnection
	if sS, err = NewConnection(erS.cfg, erS.sSChan, erS.dispatcherChan, erS.cfg.ERsCfg().SessionSConns); err != nil {
		utils.Logger.Crit(fmt.Sprintf("<%s> failed connecting to <%s>, error: <%s>",
			utils.ERs, utils.SessionS, err.Error()))
		return
	}
	erS.RLock()
	erS.ers.SetSessionSConnection(sS)
	erS.rldChan <- struct{}{}
	erS.RUnlock()
	return
}

// Shutdown stops the service
func (erS *EventReaderService) Shutdown() (err error) {
	erS.Lock()
	close(erS.stopChan)
	erS.ers = nil
	erS.Unlock()
	return
}

// IsRunning returns if the service is running
func (erS *EventReaderService) IsRunning() bool {
	erS.RLock()
	defer erS.RUnlock()
	return erS != nil && erS.ers != nil
}

// ServiceName returns the service name
func (erS *EventReaderService) ServiceName() string {
	return utils.ERs
}

// ShouldRun returns if the service should be running
func (erS *EventReaderService) ShouldRun() bool {
	return erS.cfg.ERsCfg().Enabled
}