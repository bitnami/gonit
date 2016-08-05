package monitor

import (
	"fmt"

	"github.com/bitnami/gonit/log"
)

type configValidator struct {
	SettingsDatabase map[string]string
	Success          bool
	Logger           Logger
	Checks           []interface {
		Checkable
	}
}

func newValidator() *configValidator {
	cv := configValidator{}
	cv.SettingsDatabase = map[string]string{}
	cv.Logger = log.DummyLogger()
	cv.Success = true
	return &cv
}

func (cv *configValidator) SetNamespacedConfig(namespace string, attrs map[string]string) {
	// TODO: Validate this...
}

func (cv *configValidator) SetAttribute(key, value string) {
	if cv.SettingsDatabase == nil {
		cv.SettingsDatabase = map[string]string{}
	}
	cv.SettingsDatabase[key] = value
}

func (cv *configValidator) FindCheck(id string) *interface {
	Checkable
} {
	for _, c := range cv.Checks {
		if c.GetID() == id {
			return &c
		}
	}
	return nil
}

func (cv *configValidator) AddCheck(c interface {
	Checkable
}) error {
	if cv.FindCheck(c.GetID()) != nil {
		err := fmt.Errorf("Error: Service name conflict, %s already defined", c.GetID())
		cv.Logger.Printf(err.Error())
		cv.Success = false
		return err
	}
	cv.Checks = append(cv.Checks, c)
	return nil
}
