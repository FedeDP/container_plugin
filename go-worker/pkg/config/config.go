package config

import (
	"encoding/json"
)

const defaultLabelMaxLen = 100

type SocketsEngine struct {
	Enabled bool     `json:"enabled"`
	Sockets []string `json:"sockets"`
}

type EngineCfg struct {
	SocketsEngines map[string]SocketsEngine `json:"engines"`
	LabelMaxLen    int                      `json:"label_max_len"`
}

var c EngineCfg

// Init sets cfg default values
func init() {
	c.LabelMaxLen = defaultLabelMaxLen
}

func Load(initCfg string) error {
	err := json.Unmarshal([]byte(initCfg), &c)
	if err != nil {
		return err
	}
	return nil
}

func Get() EngineCfg {
	return c
}

func GetLabelMaxLen() int {
	return c.LabelMaxLen
}
