package utils

//supported on-the-fly config change: cloudlog.xml
//watcher cannot add dir recursively
//don't add file to watcher as event cannot be receive in case file removed

import (
	log "github.com/cihub/seelog"
	"github.com/fsnotify/fsnotify"
)

const (
	LogConfigFile = "conf/log.xml"
)

var (
	watchList = []string{"./conf"}
)

func handleFsEvent(event fsnotify.Event) error {
	log.Info("file: ", event.Name)
	switch event.Name {
	case LogConfigFile:
		if event.Op&fsnotify.Write == fsnotify.Write ||
			event.Op&fsnotify.Create == fsnotify.Create {
			log.Info("handling seelog config change event:", event)
			loadLogConfig()
		}
	}
	return nil
}

func StartSeelogConfigWatcher() {
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
		log.Info("watching seelog config file change...")
		for {
			select {
			case evt := <-wch.Events:
				handleFsEvent(evt)
			case err := <-wch.Errors:
				log.Error("error:", err.Error())
			}
		}
	}()
}

func loadLogConfig() error {
	inst, e := log.LoggerFromConfigAsFile(LogConfigFile)
	if e != nil {
		log.Error("text: load seelog config error,", e)
		return e
	}
	return log.ReplaceLogger(inst)
}

func LoadSeelogConfig() error {
	return loadLogConfig()
}
