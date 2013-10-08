package app

import (
	"github.com/cloudfoundry/hm9000/models"
)

type Dea struct {
	DeaGuid string
	apps    map[int]App
}

func NewDea() Dea {
	return Dea{
		DeaGuid: models.Guid(),
		apps:    make(map[int]App, 0),
	}
}

func (dea Dea) GetApp(index int) App {
	_, ok := dea.apps[index]
	if !ok {
		dea.apps[index] = newAppForDeaGuid(dea.DeaGuid)
	}

	return dea.apps[index]
}

func (dea Dea) Heartbeat(numApps int) models.Heartbeat {
	instanceHeartbeats := make([]models.InstanceHeartbeat, 0)
	for i := 0; i < numApps; i++ {
		instanceHeartbeats = append(instanceHeartbeats, dea.GetApp(i).InstanceAtIndex(0).Heartbeat())
	}

	return models.Heartbeat{
		DeaGuid:            dea.DeaGuid,
		InstanceHeartbeats: instanceHeartbeats,
	}
}
