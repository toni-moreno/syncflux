package webui

import (
	//"github.com/go-macaron/binding"
	"github.com/toni-moreno/syncflux/pkg/agent"
	"gopkg.in/macaron.v1"
)

// NewAPICfgImportExport Import/Export REST API creator
func NewAPI(m *macaron.Macaron) error {

	//bind := binding.Bind

	m.Group("/api/", func() {
		m.Get("/health/" /*reqSignedIn,*/, HealthCluster)
		m.Get("/health/:id" /*reqSignedIn,*/, HealthID)
		m.Post("/action/:id", reqSignedIn, Action)
		m.Get("/queryactive", QueryActive)
	})

	return nil
}

func HealthCluster(ctx *Context) {
	log.Info("API: /healthcluster")

	ctx.JSON(200, agent.Cluster.GetStatus())
}

func QueryActive(ctx *Context) {
	log.Info("API: /queryactive")

	status := agent.Cluster.GetStatus()

	active := []string{}

	if status.MasterState {
		active = append(active, status.MID)
	}
	if status.SlaveState {
		active = append(active, status.SID)
	}

	ctx.JSON(200, active)
}

func HealthID(ctx *Context) {
	log.Info("Doing Action")

	ctx.JSON(200, "hola")
}

// ExportObject export object
func Action(ctx *Context) {
	id := ctx.Params(":id")

	log.Infof("Doing Action %s", id)

	ctx.JSON(200, "hola")

}
