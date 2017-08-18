package server

import (
	"broker/acl"

	log "github.com/cihub/seelog"
	"github.com/fsnotify/fsnotify"
)

var (
	watchList = []string{"./conf"}
)

func (s *Server) handleFsEvent(event fsnotify.Event) error {
	switch event.Name {
	case s.info.AclConf:
		if event.Op&fsnotify.Write == fsnotify.Write ||
			event.Op&fsnotify.Create == fsnotify.Create {
			log.Info("text:handling acl config change event:", event)
			aclconfig, err := acl.AclConfigLoad(event.Name)
			if err != nil {
				log.Error("aclconfig change failed, load acl conf error: ", err)
				return err
			}
			s.AclConfig = aclconfig
		}
	}
	return nil
}

func (s *Server) StartAclWatcher() {
	go func() {
		wch, e := fsnotify.NewWatcher()
		if e != nil {
			panic(e)
		}
		defer wch.Close()

		for _, i := range watchList {
			if err := wch.Add(i); err != nil {
				panic(err)
			}
		}
		log.Info("watching acl config file change...")
		for {
			select {
			case evt := <-wch.Events:
				s.handleFsEvent(evt)
			case err := <-wch.Errors:
				log.Error("error:", err.Error())
			}
		}
	}()
}
