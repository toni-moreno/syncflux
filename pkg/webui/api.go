package webui

import (
	//"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"
)

// NewAPICfgImportExport Import/Export REST API creator
func NewAPI(m *macaron.Macaron) error {

	//bind := binding.Bind

	m.Group("/api/", func() {
		m.Get("/health/:id" /*reqSignedIn,*/, Health)
		m.Post("/action/:id", reqSignedIn, Action)
	})

	return nil
}

func Health(ctx *Context) {
	log.Info("Doing Action")

	ctx.JSON(200, "hola")
}

// ExportObject export object
func Action(ctx *Context) {
	id := ctx.Params(":id")

	log.Infof("Doing Action %s", id)

	ctx.JSON(200, "hola")

}
