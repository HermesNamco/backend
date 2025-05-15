package main

import (
	"backend/service/reporter"
	"github.com/gin-gonic/gin"
	"log"
)

type ReportReq struct {
	text string
}

func reportHandler(c *gin.Context) {
	req := ReportReq{}
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		log.Fatal(err)
	}
	if err := reporter.NewReporter().Send(req.text); err != nil {
		log.Fatal(err)
	}
}
