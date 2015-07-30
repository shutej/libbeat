package fileout

import (
	"encoding/json"
	"time"

	"github.com/elastic/libbeat/common"
	"github.com/elastic/libbeat/logp"
	"github.com/elastic/libbeat/outputs"
)

type FileOutput struct {
	rotator logp.FileRotator
}

func (out *FileOutput) Init(config outputs.Config, topologyExpire int) error {
	out.rotator.Path = config.Path
	out.rotator.Name = config.Filename
	if out.rotator.Name == "" {
		out.rotator.Name = "packetbeat"
	}

	rotateeverybytes := uint64(config.RotateEveryKb) * 1024
	if rotateeverybytes == 0 {
		rotateeverybytes = 10 * 1024 * 1024
	}
	out.rotator.RotateEveryBytes = &rotateeverybytes

	keepfiles := config.NumberOfFiles
	if keepfiles == 0 {
		keepfiles = 7
	}
	out.rotator.KeepFiles = &keepfiles

	err := out.rotator.CreateDirectory()
	if err != nil {
		return err
	}

	err = out.rotator.CheckIfConfigSane()
	if err != nil {
		return err
	}

	return nil
}

func (out *FileOutput) PublishIPs(name string, localAddrs []string) error {
	// not supported by this output type
	return nil
}

func (out *FileOutput) GetNameByIP(ip string) string {
	// not supported by this output type
	return ""
}

func (out *FileOutput) PublishEvent(ts time.Time, event common.MapStr) error {

	json_event, err := json.Marshal(event)
	if err != nil {
		logp.Err("Fail to convert the event to JSON: %s", err)
		return err
	}

	err = out.rotator.WriteLine(json_event)
	if err != nil {
		return err
	}

	return nil
}
