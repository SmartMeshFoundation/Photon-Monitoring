package restful

import (
	"net/http"

	"fmt"

	"github.com/SmartMeshFoundation/Photon/log"

	"github.com/SmartMeshFoundation/Photon-Monitoring/models"
	"github.com/SmartMeshFoundation/Photon-Monitoring/params"
	"github.com/SmartMeshFoundation/Photon-Monitoring/verifier"
	"github.com/ant0ine/go-json-rest/rest"
)

var db *models.ModelDB
var verify verifier.DelegateVerifier

/*
Start the restful server
*/
func Start(d *models.ModelDB, v verifier.DelegateVerifier) {
	db = d
	verify = v
	api := rest.NewApi()
	api.Use(rest.DefaultDevStack...)
	router, err := rest.MakeRouter(
		rest.Post("/delegate/:delegater", Delegate),
		rest.Get("/tx/:delegater/:channel", Tx),
		rest.Get("/fee/:delegater", Fee),
	)
	if err != nil {
		log.Crit(fmt.Sprintf("maker router :%s", err))
	}
	api.SetApp(router)
	listen := fmt.Sprintf("0.0.0.0:%d", params.APIPort)
	log.Crit(fmt.Sprintf("http listen and serve :%s", http.ListenAndServe(listen, api.MakeHandler())))
}
